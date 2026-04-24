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

func TestPolicyEvaluatorReflectsPolicyEditorChanges(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	evaluator := NewPolicyEvaluator(db, config.Default())

	initial, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("initial effective policy: %v", err)
	}
	if initial.Source != PolicySourceAccountDefault || initial.Depth != config.CacheDepthMetadata {
		t.Fatalf("initial policy = %#v, want account default metadata", initial)
	}

	retention := 14
	if _, err := cache.NewSyncPolicyEditor(db).Save(ctx, cache.SyncPolicy{
		AccountID:      account.ID,
		LabelID:        "Label_1",
		Include:        true,
		Depth:          string(config.CacheDepthBody),
		RetentionDays:  &retention,
		AttachmentRule: string(config.AttachmentRuleNone),
	}); err != nil {
		t.Fatalf("save edited policy: %v", err)
	}
	edited, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("edited effective policy: %v", err)
	}
	if edited.Source != PolicySourceDBExplicit || edited.Depth != config.CacheDepthBody || valueOrZero(edited.RetentionDays) != 14 {
		t.Fatalf("edited policy = %#v, want explicit body/14", edited)
	}

	if _, err := cache.NewSyncPolicyEditor(db).ResetToDefault(ctx, account.ID, "Label_1"); err != nil {
		t.Fatalf("reset edited policy: %v", err)
	}
	reset, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("reset effective policy: %v", err)
	}
	if reset.Source != PolicySourceAccountDefault || reset.Depth != config.CacheDepthMetadata {
		t.Fatalf("reset policy = %#v, want account default metadata", reset)
	}
}

func TestPolicyEvaluatorReloadsConfigPoliciesBetweenLaunches(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)

	first := config.Default()
	first.Cache.Policies = []config.CachePolicy{
		{Account: account.Email, Label: "Label_1", Depth: config.CacheDepthBody, RetentionDays: 14, AttachmentRule: config.AttachmentRuleNone},
	}
	firstPolicy, err := NewPolicyEvaluator(db, first).EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("first effective policy: %v", err)
	}
	if firstPolicy.Source != PolicySourceConfig || firstPolicy.Depth != config.CacheDepthBody || valueOrZero(firstPolicy.RetentionDays) != 14 {
		t.Fatalf("first policy = %#v, want config body/14", firstPolicy)
	}

	second := config.Default()
	second.Cache.Policies = []config.CachePolicy{
		{Account: account.Email, Label: "Label_1", Depth: config.CacheDepthFull, RetentionDays: 365, AttachmentRule: config.AttachmentRuleAll},
	}
	secondPolicy, err := NewPolicyEvaluator(db, second).EffectiveForLabel(ctx, account.ID, account.Email, "Label_1")
	if err != nil {
		t.Fatalf("second effective policy: %v", err)
	}
	if secondPolicy.Source != PolicySourceConfig || secondPolicy.Depth != config.CacheDepthFull || valueOrZero(secondPolicy.RetentionDays) != 365 || secondPolicy.AttachmentRule != config.AttachmentRuleAll {
		t.Fatalf("second policy = %#v, want reloaded config full/365/all", secondPolicy)
	}
}

func TestPolicyEvaluatorMapsConfigPolicyAliasesToAccounts(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	accounts := cache.NewAccountRepository(db)
	work, err := accounts.Create(ctx, cache.CreateAccountParams{Email: "work@example.com"})
	if err != nil {
		t.Fatalf("create work account: %v", err)
	}
	personal, err := accounts.Create(ctx, cache.CreateAccountParams{Email: "personal@example.com"})
	if err != nil {
		t.Fatalf("create personal account: %v", err)
	}

	cfg := config.Default()
	cfg.Accounts = map[string]config.AccountConfig{
		"work":     {Email: work.Email},
		"personal": {ID: personal.ID},
	}
	cfg.Cache.Policies = []config.CachePolicy{
		{Account: "work", Label: "Label_1", Depth: config.CacheDepthBody, RetentionDays: 30, AttachmentRule: config.AttachmentRuleNone},
		{Account: "personal", Label: "Label_1", Depth: config.CacheDepthFull, RetentionDays: 365, AttachmentRule: config.AttachmentRuleAll, AttachmentMaxMB: 25},
	}
	evaluator := NewPolicyEvaluator(db, cfg)

	workPolicy, err := evaluator.EffectiveForLabel(ctx, work.ID, work.Email, "Label_1")
	if err != nil {
		t.Fatalf("work policy: %v", err)
	}
	personalPolicy, err := evaluator.EffectiveForLabel(ctx, personal.ID, personal.Email, "Label_1")
	if err != nil {
		t.Fatalf("personal policy: %v", err)
	}
	if workPolicy.Depth != config.CacheDepthBody || valueOrZero(workPolicy.RetentionDays) != 30 || workPolicy.AttachmentRule != config.AttachmentRuleNone {
		t.Fatalf("work policy = %#v, want body/30/none", workPolicy)
	}
	if personalPolicy.Depth != config.CacheDepthFull || valueOrZero(personalPolicy.RetentionDays) != 365 || personalPolicy.AttachmentRule != config.AttachmentRuleAll || valueOrZero(personalPolicy.AttachmentMaxMB) != 25 {
		t.Fatalf("personal policy = %#v, want full/365/all/25", personalPolicy)
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
