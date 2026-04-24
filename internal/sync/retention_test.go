package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/jmoiron/sqlx"
)

func TestRetentionSweeperPrunesOnlyExpiredAcrossAllLabels(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	attachmentPath := seedRetentionRows(t, db, account.ID, now)

	result, err := NewRetentionSweeper(db, config.Default()).Sweep(ctx, account, now)
	if err != nil {
		t.Fatalf("sweep retention: %v", err)
	}
	if result.Checked != 6 || result.Purged != 3 || result.ExcludedPurged != 1 {
		t.Fatalf("result = %#v, want checked 6 purged 3 with one exclusion", result)
	}
	for _, id := range []string{"old-inbox", "multi-purge", "excluded-recent"} {
		if _, err := cache.NewMessageRepository(db).Get(ctx, account.ID, id); !errors.Is(err, cache.ErrMessageNotFound) {
			t.Fatalf("message %s error = %v, want purged", id, err)
		}
	}
	for _, id := range []string{"recent-inbox", "multi-keep", "nolimit-old"} {
		if _, err := cache.NewMessageRepository(db).Get(ctx, account.ID, id); err != nil {
			t.Fatalf("message %s was not retained: %v", id, err)
		}
	}
	if _, err := os.Stat(attachmentPath); !os.IsNotExist(err) {
		t.Fatalf("attachment stat = %v, want removed by retention purge", err)
	}
}

func TestRetentionScheduleRunsOnStartupAndThenDaily(t *testing.T) {
	schedule := NewRetentionSchedule()
	start := time.Date(2026, 4, 24, 8, 0, 0, 0, time.UTC)
	if !schedule.ShouldRun("acct", start) {
		t.Fatal("first retention check should run on startup")
	}
	if schedule.ShouldRun("acct", start.Add(23*time.Hour)) {
		t.Fatal("retention check reran before 24 hours")
	}
	if !schedule.ShouldRun("acct", start.Add(24*time.Hour)) {
		t.Fatal("retention check did not run after 24 hours")
	}
}

func seedRetentionRows(t *testing.T, db *sqlx.DB, accountID string, now time.Time) string {
	t.Helper()

	ctx := context.Background()
	labels := cache.NewLabelRepository(db)
	for _, label := range []cache.Label{
		{AccountID: accountID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: accountID, ID: "Long", Name: "Long", Type: "user"},
		{AccountID: accountID, ID: "NoLimit", Name: "NoLimit", Type: "user"},
	} {
		if err := labels.Upsert(ctx, label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}
	retention365 := 365
	policies := cache.NewSyncPolicyRepository(db)
	if err := policies.Upsert(ctx, cache.SyncPolicy{AccountID: accountID, LabelID: "Long", Include: true, Depth: "metadata", RetentionDays: &retention365, AttachmentRule: "none"}); err != nil {
		t.Fatalf("upsert long policy: %v", err)
	}
	if err := policies.Upsert(ctx, cache.SyncPolicy{AccountID: accountID, LabelID: "NoLimit", Include: true, Depth: "metadata", AttachmentRule: "none"}); err != nil {
		t.Fatalf("upsert no-limit policy: %v", err)
	}
	if err := cache.NewCacheExclusionRepository(db).Upsert(ctx, cache.CacheExclusion{AccountID: accountID, MatchType: "sender", MatchValue: "private@example.com", CreatedAt: now}); err != nil {
		t.Fatalf("upsert exclusion: %v", err)
	}

	attachmentPath := filepath.Join(t.TempDir(), "excluded.pdf")
	if err := os.WriteFile(attachmentPath, []byte("attachment"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	fixtures := []struct {
		id     string
		labels []string
		from   string
		age    int
		attach bool
	}{
		{id: "old-inbox", labels: []string{"INBOX"}, from: "ada@example.com", age: 120},
		{id: "recent-inbox", labels: []string{"INBOX"}, from: "ada@example.com", age: 10},
		{id: "multi-keep", labels: []string{"INBOX", "Long"}, from: "ada@example.com", age: 120},
		{id: "multi-purge", labels: []string{"INBOX", "Long"}, from: "ada@example.com", age: 400},
		{id: "nolimit-old", labels: []string{"NoLimit"}, from: "ada@example.com", age: 800},
		{id: "excluded-recent", labels: []string{"INBOX"}, from: "private@example.com", age: 10, attach: true},
	}
	for _, fixture := range fixtures {
		when := now.AddDate(0, 0, -fixture.age)
		if err := cache.NewThreadRepository(db).Upsert(ctx, cache.Thread{AccountID: accountID, ID: fixture.id + "-thread", LastMessageDate: &when}); err != nil {
			t.Fatalf("upsert thread: %v", err)
		}
		if err := cache.NewMessageRepository(db).Upsert(ctx, cache.Message{
			AccountID:    accountID,
			ID:           fixture.id,
			ThreadID:     fixture.id + "-thread",
			FromAddr:     fixture.from,
			Subject:      fixture.id,
			SizeBytes:    100,
			InternalDate: &when,
		}); err != nil {
			t.Fatalf("upsert message %s: %v", fixture.id, err)
		}
		for _, labelID := range fixture.labels {
			if err := cache.NewMessageLabelRepository(db).Upsert(ctx, cache.MessageLabel{AccountID: accountID, MessageID: fixture.id, LabelID: labelID}); err != nil {
				t.Fatalf("upsert label %s for %s: %v", labelID, fixture.id, err)
			}
		}
		if fixture.attach {
			if err := cache.NewAttachmentRepository(db).Upsert(ctx, cache.Attachment{AccountID: accountID, MessageID: fixture.id, PartID: "1", Filename: "excluded.pdf", SizeBytes: 50, LocalPath: attachmentPath}); err != nil {
				t.Fatalf("upsert attachment: %v", err)
			}
		}
	}
	return attachmentPath
}
