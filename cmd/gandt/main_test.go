package main

import (
	"context"
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
