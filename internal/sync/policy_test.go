package sync

import (
	"context"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/jmoiron/sqlx"
)

func TestPolicyEvaluatorPrecedence(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	policies := cache.NewSyncPolicyRepository(db)

	cfg := config.Default()
	cfg.Cache.Defaults.Depth = config.CacheDepthBody
	cfg.Cache.Defaults.RetentionDays = 14
	cfg.Cache.Policies = []config.CachePolicy{
		{
			Account:         account.ID,
			Label:           "Label_1",
			Depth:           config.CacheDepthFull,
			RetentionDays:   1825,
			AttachmentRule:  config.AttachmentRuleAll,
			AttachmentMaxMB: 25,
		},
	}
	evaluator := NewPolicyEvaluator(db, cfg)

	fromConfig, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("effective config policy: %v", err)
	}
	if fromConfig.Source != PolicySourceConfig || fromConfig.Depth != config.CacheDepthFull || valueOrZero(fromConfig.RetentionDays) != 1825 {
		t.Fatalf("config policy = %#v, want config full/1825", fromConfig)
	}

	retention := 30
	attachmentMax := 5
	if err := policies.Upsert(ctx, cache.SyncPolicy{
		AccountID:       account.ID,
		LabelID:         "Label_1",
		Include:         true,
		Depth:           string(config.CacheDepthMetadata),
		RetentionDays:   &retention,
		AttachmentRule:  string(config.AttachmentRuleUnderSize),
		AttachmentMaxMB: &attachmentMax,
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert db policy: %v", err)
	}
	fromDB, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("effective db policy: %v", err)
	}
	if fromDB.Source != PolicySourceDBExplicit || fromDB.Depth != config.CacheDepthMetadata || valueOrZero(fromDB.RetentionDays) != 30 {
		t.Fatalf("db policy = %#v, want explicit metadata/30", fromDB)
	}

	fromDefault, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_2")
	if err != nil {
		t.Fatalf("effective account default: %v", err)
	}
	if fromDefault.Source != PolicySourceAccountDefault || fromDefault.Depth != config.CacheDepthMetadata || valueOrZero(fromDefault.RetentionDays) != 365 {
		t.Fatalf("account default = %#v, want seeded metadata/365", fromDefault)
	}

	if err := policies.Delete(ctx, account.ID, cache.DefaultPolicyLabelID); err != nil {
		t.Fatalf("delete account default: %v", err)
	}
	fromGlobal, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_2")
	if err != nil {
		t.Fatalf("effective global default: %v", err)
	}
	if fromGlobal.Source != PolicySourceGlobalDefault || fromGlobal.Depth != config.CacheDepthBody || valueOrZero(fromGlobal.RetentionDays) != 14 {
		t.Fatalf("global default = %#v, want config default body/14", fromGlobal)
	}
}

func TestPolicyEvaluatorCombinesMultiLabelMaximums(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	policies := cache.NewSyncPolicyRepository(db)

	retention := 1825
	if err := policies.Upsert(ctx, cache.SyncPolicy{
		AccountID:      account.ID,
		LabelID:        "Label_1",
		Include:        true,
		Depth:          string(config.CacheDepthBody),
		RetentionDays:  &retention,
		AttachmentRule: string(config.AttachmentRuleAll),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert label policy: %v", err)
	}

	decision, err := NewPolicyEvaluator(db, config.Default()).Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         "Ada <ada@example.com>",
		LabelIDs:     []string{"INBOX", "Label_1", "Label_1"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Persist || decision.Depth != config.CacheDepthFull {
		t.Fatalf("decision = %#v, want persistent full depth", decision)
	}
	if valueOrZero(decision.RetentionDays) != 1825 {
		t.Fatalf("retention = %v, want longest retention 1825", decision.RetentionDays)
	}
	if decision.AttachmentRule != config.AttachmentRuleAll {
		t.Fatalf("attachment rule = %s, want all", decision.AttachmentRule)
	}
	if len(decision.Policies) != 2 {
		t.Fatalf("policies = %#v, want unique labels only", decision.Policies)
	}
}

func TestPolicyEvaluatorExclusionsNeverPersist(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	exclusions := cache.NewCacheExclusionRepository(db)
	if err := exclusions.Upsert(ctx, cache.CacheExclusion{
		AccountID:  account.ID,
		MatchType:  "sender",
		MatchValue: "sensitive@example.com",
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert sender exclusion: %v", err)
	}

	decision, err := NewPolicyEvaluator(db, config.Default()).Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         "Sensitive <sensitive@example.com>",
		LabelIDs:     []string{"INBOX"},
	})
	if err != nil {
		t.Fatalf("evaluate sender exclusion: %v", err)
	}
	if !decision.Excluded || decision.Persist || decision.Depth != config.CacheDepthNone {
		t.Fatalf("sender exclusion decision = %#v, want never-persist none", decision)
	}

	cfg := config.Default()
	cfg.Cache.Exclusions = []config.CacheExclusion{
		{Account: account.Email, MatchType: "domain", MatchValue: "private.example"},
		{Account: account.Email, MatchType: "label", MatchValue: "PRIVATE"},
	}
	evaluator := NewPolicyEvaluator(db, cfg)
	domainDecision, err := evaluator.Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         "sender@private.example",
		LabelIDs:     []string{"INBOX"},
	})
	if err != nil {
		t.Fatalf("evaluate domain exclusion: %v", err)
	}
	if !domainDecision.Excluded || domainDecision.Persist {
		t.Fatalf("domain decision = %#v, want excluded", domainDecision)
	}

	labelDecision, err := evaluator.Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         "sender@example.com",
		LabelIDs:     []string{"PRIVATE", "INBOX"},
	})
	if err != nil {
		t.Fatalf("evaluate label exclusion: %v", err)
	}
	if !labelDecision.Excluded || labelDecision.Persist {
		t.Fatalf("label decision = %#v, want excluded", labelDecision)
	}
}

func migratedSyncTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	ctx := context.Background()
	db, err := cache.OpenPath(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
	})
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	return db
}

func seedSyncAccount(t *testing.T, db *sqlx.DB) cache.Account {
	t.Helper()

	account, err := cache.NewAccountRepository(db).Create(context.Background(), cache.CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
