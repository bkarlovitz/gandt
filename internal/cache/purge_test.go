package cache

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestCachePurgePlanFiltersAndDryRunDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	accountID := seedPurgeRows(t, db, now)

	plan, err := NewCachePurgeService(db).Plan(ctx, CachePurgeFilter{
		AccountID:     accountID,
		LabelID:       "INBOX",
		OlderThanDays: 30,
		From:          "Ada <ada@example.com>",
		DryRun:        true,
	}, now)
	if err != nil {
		t.Fatalf("plan purge: %v", err)
	}
	if !plan.Filter.DryRun || plan.MessageCount != 1 || plan.BodyCount != 1 || plan.AttachmentCount != 1 || plan.MessageKeys[0].MessageID != "old-inbox-ada" {
		t.Fatalf("plan = %#v, want one old inbox sender match", plan)
	}
	if plan.EstimatedBytes <= 100 {
		t.Fatalf("estimated bytes = %d, want message/body/attachment estimate", plan.EstimatedBytes)
	}

	if _, err := NewMessageRepository(db).Get(ctx, accountID, "old-inbox-ada"); err != nil {
		t.Fatalf("dry run mutated message: %v", err)
	}
	if _, err := NewAccountRepository(db).Get(ctx, accountID); err != nil {
		t.Fatalf("dry run removed account registry: %v", err)
	}
}

func TestCachePurgePlanSupportsLabelAndAccountScopes(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	accountID := seedPurgeRows(t, db, now)

	plan, err := NewCachePurgeService(db).Plan(ctx, CachePurgeFilter{AccountID: accountID, LabelID: "Receipts"}, now)
	if err != nil {
		t.Fatalf("plan label purge: %v", err)
	}
	if plan.MessageCount != 1 || plan.MessageKeys[0].MessageID != "old-receipts" {
		t.Fatalf("label plan = %#v, want only receipts message in account scope", plan)
	}

	allAccounts, err := NewCachePurgeService(db).Plan(ctx, CachePurgeFilter{LabelID: "INBOX", OlderThanDays: 30}, now)
	if err != nil {
		t.Fatalf("plan all-account purge: %v", err)
	}
	if allAccounts.MessageCount != 2 {
		t.Fatalf("all-account plan = %#v, want old inbox messages from both accounts", allAccounts)
	}
}

func seedPurgeRows(t *testing.T, db *sqlx.DB, now time.Time) string {
	t.Helper()

	ctx := context.Background()
	accounts := NewAccountRepository(db)
	first, err := accounts.Create(ctx, CreateAccountParams{Email: "first@example.com"})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	second, err := accounts.Create(ctx, CreateAccountParams{Email: "second@example.com"})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}
	for _, accountID := range []string{first.ID, second.ID} {
		for _, label := range []Label{
			{AccountID: accountID, ID: "INBOX", Name: "Inbox", Type: "system"},
			{AccountID: accountID, ID: "Receipts", Name: "Receipts", Type: "user"},
		} {
			if err := NewLabelRepository(db).Upsert(ctx, label); err != nil {
				t.Fatalf("upsert label %s: %v", label.ID, err)
			}
		}
	}

	body := "cached body"
	old := now.AddDate(0, 0, -45)
	recent := now.AddDate(0, 0, -3)
	fixtures := []struct {
		accountID string
		id        string
		labelID   string
		from      string
		body      *string
		when      time.Time
	}{
		{accountID: first.ID, id: "old-inbox-ada", labelID: "INBOX", from: "Ada <ada@example.com>", body: &body, when: old},
		{accountID: first.ID, id: "recent-inbox-ada", labelID: "INBOX", from: "ada@example.com", when: recent},
		{accountID: first.ID, id: "old-receipts", labelID: "Receipts", from: "receipts@example.com", when: old},
		{accountID: second.ID, id: "old-second-inbox", labelID: "INBOX", from: "ada@example.com", when: old},
	}
	for _, fixture := range fixtures {
		if err := NewThreadRepository(db).Upsert(ctx, Thread{AccountID: fixture.accountID, ID: fixture.id + "-thread", LastMessageDate: &fixture.when}); err != nil {
			t.Fatalf("upsert thread: %v", err)
		}
		message := Message{
			AccountID:    fixture.accountID,
			ID:           fixture.id,
			ThreadID:     fixture.id + "-thread",
			FromAddr:     fixture.from,
			Subject:      fixture.id,
			SizeBytes:    100,
			BodyPlain:    fixture.body,
			InternalDate: &fixture.when,
		}
		if fixture.body != nil {
			message.CachedAt = &fixture.when
		}
		if err := NewMessageRepository(db).Upsert(ctx, message); err != nil {
			t.Fatalf("upsert message %s: %v", fixture.id, err)
		}
		if err := NewMessageLabelRepository(db).Upsert(ctx, MessageLabel{AccountID: fixture.accountID, MessageID: fixture.id, LabelID: fixture.labelID}); err != nil {
			t.Fatalf("upsert label mapping %s: %v", fixture.id, err)
		}
	}
	if err := NewAttachmentRepository(db).Upsert(ctx, Attachment{AccountID: first.ID, MessageID: "old-inbox-ada", PartID: "1", Filename: "one.pdf", SizeBytes: 50}); err != nil {
		t.Fatalf("upsert attachment: %v", err)
	}
	return first.ID
}
