package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestOutboxRepositoryPersistsPendingAndTransitions(t *testing.T) {
	ctx := context.Background()
	db := openMigratedCache(t)
	defer db.Close()
	seedAccount(t, db, "acct-1", "me@example.com")
	repo := NewOutboxRepository(db)
	queuedAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	message, err := repo.Queue(ctx, OutboxMessage{
		ID:        "out-1",
		AccountID: "acct-1",
		RawRFC822: []byte("raw"),
		QueuedAt:  queuedAt,
	})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if message.Status != OutboxStatusPending {
		t.Fatalf("status = %q, want pending", message.Status)
	}
	pending, err := repo.Pending(ctx, "acct-1", 10)
	if err != nil || len(pending) != 1 || string(pending[0].RawRFC822) != "raw" {
		t.Fatalf("pending = %#v err=%v", pending, err)
	}

	if err := repo.MarkRetry(ctx, "out-1", "offline"); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	got, err := repo.Get(ctx, "out-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Attempts != 1 || got.LastError != "offline" || got.Status != OutboxStatusPending {
		t.Fatalf("retry row = %#v", got)
	}

	if err := repo.MarkSent(ctx, "out-1"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	got, _ = repo.Get(ctx, "out-1")
	if got.Status != OutboxStatusSent || got.LastError != "" {
		t.Fatalf("sent row = %#v", got)
	}
}

func openMigratedCache(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx := context.Background()
	db, err := OpenPath(ctx, filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	return db
}

func seedAccount(t *testing.T, db *sqlx.DB, id string, email string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO accounts (id, email, added_at) VALUES (?, ?, ?)`, id, email, time.Now().UTC()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
}
