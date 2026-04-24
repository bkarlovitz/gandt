package sync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/jmoiron/sqlx"
)

type fakeOutboxSender struct {
	err  error
	sent [][]byte
}

func (s *fakeOutboxSender) SendMessage(_ context.Context, raw []byte) error {
	s.sent = append(s.sent, append([]byte{}, raw...))
	return s.err
}

func TestOutboxRetryerMarksSent(t *testing.T) {
	ctx := context.Background()
	db := openSyncOutboxCache(t)
	defer db.Close()
	seedSyncOutboxAccount(t, db, "acct-1", "me@example.com")
	repo := cache.NewOutboxRepository(db)
	_, err := repo.Queue(ctx, cache.OutboxMessage{
		ID:        "out-1",
		AccountID: "acct-1",
		RawRFC822: []byte("raw"),
		QueuedAt:  time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	sender := &fakeOutboxSender{}
	result, err := OutboxRetryer{
		Repository:  repo,
		Sender:      sender,
		BaseBackoff: time.Second,
		Now:         func() time.Time { return time.Date(2026, 4, 24, 12, 0, 2, 0, time.UTC) },
	}.Retry(ctx, "acct-1")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if result.Sent != 1 || len(sender.sent) != 1 {
		t.Fatalf("result=%#v sent=%d", result, len(sender.sent))
	}
	got, _ := repo.Get(ctx, "out-1")
	if got.Status != cache.OutboxStatusSent {
		t.Fatalf("status = %q, want sent", got.Status)
	}
}

func TestOutboxRetryerFailureAndBackoff(t *testing.T) {
	ctx := context.Background()
	db := openSyncOutboxCache(t)
	defer db.Close()
	seedSyncOutboxAccount(t, db, "acct-1", "me@example.com")
	repo := cache.NewOutboxRepository(db)
	queuedAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if _, err := repo.Queue(ctx, cache.OutboxMessage{ID: "out-1", AccountID: "acct-1", RawRFC822: []byte("raw"), QueuedAt: queuedAt}); err != nil {
		t.Fatalf("queue: %v", err)
	}

	result, err := OutboxRetryer{
		Repository:  repo,
		Sender:      &fakeOutboxSender{err: errors.New("still offline")},
		BaseBackoff: time.Second,
		MaxAttempts: 2,
		Now:         func() time.Time { return queuedAt.Add(500 * time.Millisecond) },
	}.Retry(ctx, "acct-1")
	if err != nil {
		t.Fatalf("retry before due: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("result = %#v, want skipped", result)
	}

	result, err = OutboxRetryer{
		Repository:  repo,
		Sender:      &fakeOutboxSender{err: errors.New("still offline")},
		BaseBackoff: time.Second,
		MaxAttempts: 2,
		Now:         func() time.Time { return queuedAt.Add(2 * time.Second) },
	}.Retry(ctx, "acct-1")
	if err != nil {
		t.Fatalf("retry failure: %v", err)
	}
	got, _ := repo.Get(ctx, "out-1")
	if result.Failed != 0 || got.Attempts != 1 || got.Status != cache.OutboxStatusPending {
		t.Fatalf("after retry result=%#v row=%#v", result, got)
	}

	result, err = OutboxRetryer{
		Repository:  repo,
		Sender:      &fakeOutboxSender{err: errors.New("permanent")},
		BaseBackoff: time.Second,
		MaxAttempts: 2,
		Now:         func() time.Time { return queuedAt.Add(4 * time.Second) },
	}.Retry(ctx, "acct-1")
	if err != nil {
		t.Fatalf("retry permanent: %v", err)
	}
	got, _ = repo.Get(ctx, "out-1")
	if result.Failed != 1 || got.Status != cache.OutboxStatusFailed || got.Attempts != 2 {
		t.Fatalf("final result=%#v row=%#v", result, got)
	}
}

func openSyncOutboxCache(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx := context.Background()
	db, err := cache.OpenPath(ctx, filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	return db
}

func seedSyncOutboxAccount(t *testing.T, db *sqlx.DB, id string, email string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO accounts (id, email, added_at) VALUES (?, ?, ?)`, id, email, time.Now().UTC()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
}
