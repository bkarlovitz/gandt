package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/ui"
	"github.com/jmoiron/sqlx"
)

func TestCachedThreadLoadReturnsSelectedCachedBody(t *testing.T) {
	ctx := context.Background()
	db := migratedMainTestDB(t)
	account := seedMainTestAccount(t, db)
	body := "cached body"
	when := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	if err := cache.NewThreadRepository(db).Upsert(ctx, cache.Thread{AccountID: account.ID, ID: "thread-1", LastMessageDate: &when}); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	if err := cache.NewMessageRepository(db).Upsert(ctx, cache.Message{
		AccountID:    account.ID,
		ID:           "message-1",
		ThreadID:     "thread-1",
		FromAddr:     "Ada <ada@example.com>",
		Subject:      "Cached",
		BodyPlain:    &body,
		InternalDate: &when,
	}); err != nil {
		t.Fatalf("upsert message: %v", err)
	}

	result, ok, err := cachedThreadLoad(ctx, db, config.Default(), account.ID, ui.Message{ID: "message-1", ThreadID: "thread-1"})
	if err != nil {
		t.Fatalf("cached load: %v", err)
	}
	if !ok || result.CacheState != "cached" || len(result.Body) != 1 || result.Body[0] != "cached body" {
		t.Fatalf("result = %#v ok=%v, want cached selected body", result, ok)
	}
}

func TestCachedThreadLoadMissesWhenSelectedBodyIsMissing(t *testing.T) {
	ctx := context.Background()
	db := migratedMainTestDB(t)
	account := seedMainTestAccount(t, db)
	olderBody := "older cached body"
	when := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	if err := cache.NewThreadRepository(db).Upsert(ctx, cache.Thread{AccountID: account.ID, ID: "thread-1", LastMessageDate: &when}); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	for _, message := range []cache.Message{
		{AccountID: account.ID, ID: "message-1", ThreadID: "thread-1", BodyPlain: &olderBody, InternalDate: &when},
		{AccountID: account.ID, ID: "message-2", ThreadID: "thread-1", InternalDate: &when},
	} {
		if err := cache.NewMessageRepository(db).Upsert(ctx, message); err != nil {
			t.Fatalf("upsert message %s: %v", message.ID, err)
		}
	}

	_, ok, err := cachedThreadLoad(ctx, db, config.Default(), account.ID, ui.Message{ID: "message-2", ThreadID: "thread-1"})
	if err != nil {
		t.Fatalf("cached load: %v", err)
	}
	if ok {
		t.Fatalf("cache load hit for selected metadata-only message")
	}
}

func TestCachePolicyStorePersistsRows(t *testing.T) {
	ctx := context.Background()
	paths := config.Paths{DataDir: t.TempDir()}
	db, err := cache.Open(ctx, paths)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer db.Close()
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	account := seedMainTestAccount(t, db)
	store := buildCachePolicyStore(paths, config.Default())

	table, err := store.LoadCachePolicies()
	if err != nil {
		t.Fatalf("load cache policies: %v", err)
	}
	row, ok := findPolicyRow(table.Rows, "INBOX")
	if !ok {
		t.Fatalf("missing inbox policy row: %#v", table.Rows)
	}
	row.Depth = "full"
	row.AttachmentRule = "all"
	row.AttachmentMaxMB = nil
	saved, err := store.SaveCachePolicy(row)
	if err != nil {
		t.Fatalf("save cache policy: %v", err)
	}
	if !saved.Explicit || saved.Depth != "full" || saved.AttachmentRule != "all" {
		t.Fatalf("saved row = %#v, want explicit full/all", saved)
	}
	persisted, err := cache.NewSyncPolicyRepository(db).Get(ctx, account.ID, "INBOX")
	if err != nil {
		t.Fatalf("get persisted policy: %v", err)
	}
	if persisted.Depth != "full" || persisted.AttachmentRule != "all" {
		t.Fatalf("persisted policy = %#v, want full/all", persisted)
	}

	reset, err := store.ResetCachePolicy(saved)
	if err != nil {
		t.Fatalf("reset cache policy: %v", err)
	}
	if reset.Explicit || reset.LabelID != "INBOX" {
		t.Fatalf("reset row = %#v, want inherited label row", reset)
	}
	if _, err := cache.NewSyncPolicyRepository(db).Get(ctx, account.ID, "INBOX"); !errors.Is(err, cache.ErrSyncPolicyNotFound) {
		t.Fatalf("get reset policy error = %v, want ErrSyncPolicyNotFound", err)
	}
}

func TestCachePurgeStoreRejectsUnknownAccount(t *testing.T) {
	ctx := context.Background()
	paths := config.Paths{DataDir: t.TempDir()}
	db, err := cache.Open(ctx, paths)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	seedMainTestAccount(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	_, err = buildCachePurgeStore(paths).PlanCachePurge(ui.CachePurgeRequest{Account: "missing@example.com", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), `account "missing@example.com" not found`) {
		t.Fatalf("plan purge error = %v, want unknown account error", err)
	}
}

func findPolicyRow(rows []ui.CachePolicyRow, labelID string) (ui.CachePolicyRow, bool) {
	for _, row := range rows {
		if row.LabelID == labelID {
			return row, true
		}
	}
	return ui.CachePolicyRow{}, false
}

func migratedMainTestDB(t *testing.T) *sqlx.DB {
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

func TestResolveRefreshAccountSupportsActiveDefaultAndInvalidAccount(t *testing.T) {
	accounts := []cache.Account{
		{ID: "acct-1", Email: "one@example.com"},
		{ID: "acct-2", Email: "two@example.com"},
	}

	got, err := resolveRefreshAccount(accounts, "")
	if err != nil {
		t.Fatalf("default account: %v", err)
	}
	if got.ID != "acct-1" {
		t.Fatalf("default account = %#v, want first account", got)
	}
	got, err = resolveRefreshAccount(accounts, "TWO@example.com")
	if err != nil {
		t.Fatalf("email account: %v", err)
	}
	if got.ID != "acct-2" {
		t.Fatalf("email account = %#v, want second account", got)
	}
	got, err = resolveRefreshAccount(accounts, "acct-2")
	if err != nil {
		t.Fatalf("id account: %v", err)
	}
	if got.Email != "two@example.com" {
		t.Fatalf("id account = %#v, want second account", got)
	}
	if _, err := resolveRefreshAccount(accounts, "missing@example.com"); err == nil || !strings.Contains(err.Error(), "missing@example.com") {
		t.Fatalf("missing account err = %v, want unknown account", err)
	}
}

func TestLoadInitialAccountsThreeAccountStartupUnderPRDTarget(t *testing.T) {
	ctx := context.Background()
	paths := config.Paths{DataDir: t.TempDir()}
	db, err := cache.Open(ctx, paths)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	for i := 0; i < 3; i++ {
		account, err := cache.NewAccountRepository(db).Create(ctx, cache.CreateAccountParams{Email: fmt.Sprintf("acct-%d@example.com", i)})
		if err != nil {
			t.Fatalf("create account %d: %v", i, err)
		}
		if err := seedMainAccountMessages(ctx, db, account, 250); err != nil {
			t.Fatalf("seed account %d messages: %v", i, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	start := time.Now()
	accounts, ok := loadInitialAccounts(paths, config.Default())
	elapsed := time.Since(start)
	if !ok || len(accounts) != 3 {
		t.Fatalf("accounts ok=%v len=%d, want three loaded accounts", ok, len(accounts))
	}
	if elapsed >= 300*time.Millisecond {
		t.Fatalf("startup load took %s, want below PRD target 300ms", elapsed)
	}
}

func seedMainTestAccount(t *testing.T, db *sqlx.DB) cache.Account {
	t.Helper()

	ctx := context.Background()
	account, err := cache.NewAccountRepository(db).Create(ctx, cache.CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	for _, label := range []cache.Label{
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "UNREAD", Name: "Unread", Type: "system"},
	} {
		if err := cache.NewLabelRepository(db).Upsert(ctx, label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}
	return account
}

func seedMainAccountMessages(ctx context.Context, db *sqlx.DB, account cache.Account, count int) error {
	if err := cache.NewLabelRepository(db).Upsert(ctx, cache.Label{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system", Unread: count / 3, Total: count}); err != nil {
		return err
	}
	when := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		threadID := fmt.Sprintf("thread-%03d", i)
		messageID := fmt.Sprintf("message-%03d", i)
		if err := cache.NewThreadRepository(db).Upsert(ctx, cache.Thread{AccountID: account.ID, ID: threadID, LastMessageDate: &when}); err != nil {
			return err
		}
		body := "cached body"
		if err := cache.NewMessageRepository(db).Upsert(ctx, cache.Message{
			AccountID:    account.ID,
			ID:           messageID,
			ThreadID:     threadID,
			FromAddr:     "Sender <sender@example.com>",
			Subject:      fmt.Sprintf("Subject %03d", i),
			Snippet:      "Cached snippet",
			BodyPlain:    &body,
			InternalDate: &when,
		}); err != nil {
			return err
		}
		if err := cache.NewMessageLabelRepository(db).Upsert(ctx, cache.MessageLabel{AccountID: account.ID, MessageID: messageID, LabelID: "INBOX"}); err != nil {
			return err
		}
	}
	return nil
}
