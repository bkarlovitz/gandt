package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestCacheExclusionServicePreviewMatchesSenderDomainAndLabel(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accountID := seedExclusionServiceRows(t, db)
	service := NewCacheExclusionService(db)

	senderPlan, err := service.PreviewPurge(ctx, CacheExclusion{AccountID: accountID, MatchType: "sender", MatchValue: "Ada <ada@example.com>"})
	if err != nil {
		t.Fatalf("preview sender: %v", err)
	}
	if senderPlan.Exclusion.MatchValue != "ada@example.com" || senderPlan.MessageCount != 1 || senderPlan.BodyCount != 1 || senderPlan.AttachmentCount != 1 {
		t.Fatalf("sender plan = %#v, want normalized one cached message with attachment", senderPlan)
	}

	domainPlan, err := service.PreviewPurge(ctx, CacheExclusion{AccountID: accountID, MatchType: "domain", MatchValue: "@private.example"})
	if err != nil {
		t.Fatalf("preview domain: %v", err)
	}
	if domainPlan.Exclusion.MatchValue != "private.example" || domainPlan.MessageCount != 1 || domainPlan.BodyCount != 0 {
		t.Fatalf("domain plan = %#v, want one metadata-only domain match", domainPlan)
	}

	labelPlan, err := service.PreviewPurge(ctx, CacheExclusion{AccountID: accountID, MatchType: "label", MatchValue: "Sensitive"})
	if err != nil {
		t.Fatalf("preview label: %v", err)
	}
	if labelPlan.MessageCount != 1 || labelPlan.MessageKeys[0].MessageID != "sensitive-msg" {
		t.Fatalf("label plan = %#v, want sensitive-msg", labelPlan)
	}
}

func TestCacheExclusionServiceConfirmPurgeDeletesMatchingRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accountID := seedExclusionServiceRows(t, db)
	service := NewCacheExclusionService(db)

	result, err := service.ConfirmPurge(ctx, CacheExclusion{AccountID: accountID, MatchType: "sender", MatchValue: "ada@example.com"})
	if err != nil {
		t.Fatalf("confirm purge: %v", err)
	}
	if result.DeletedMessages != 1 || result.Plan.MessageCount != 1 {
		t.Fatalf("purge result = %#v, want one deleted message", result)
	}
	if _, err := NewMessageRepository(db).Get(ctx, accountID, "ada-msg"); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("get purged message error = %v, want ErrMessageNotFound", err)
	}
	assertRowCount(t, db, "attachments", accountID, 0)

	listed, err := NewCacheExclusionRepository(db).List(ctx, accountID)
	if err != nil {
		t.Fatalf("list exclusions: %v", err)
	}
	if len(listed) != 1 || listed[0].MatchType != "sender" || listed[0].MatchValue != "ada@example.com" {
		t.Fatalf("listed exclusions = %#v, want normalized sender exclusion", listed)
	}
}

func TestCacheExclusionServiceRejectsInvalidExclusion(t *testing.T) {
	_, err := NormalizeCacheExclusion(CacheExclusion{AccountID: "acct", MatchType: "subject", MatchValue: "secret"})
	if !errors.Is(err, ErrInvalidCacheExclusion) {
		t.Fatalf("normalize error = %v, want ErrInvalidCacheExclusion", err)
	}
}

func seedExclusionServiceRows(t *testing.T, db *sqlx.DB) string {
	t.Helper()

	ctx := context.Background()
	account, err := NewAccountRepository(db).Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	labels := NewLabelRepository(db)
	for _, label := range []Label{
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "Sensitive", Name: "Sensitive", Type: "user"},
	} {
		if err := labels.Upsert(ctx, label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}

	when := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	body := "cached body"
	fixtures := []struct {
		id     string
		from   string
		label  string
		body   *string
		size   int
		cached bool
	}{
		{id: "ada-msg", from: "Ada <ada@example.com>", label: "INBOX", body: &body, size: 100, cached: true},
		{id: "domain-msg", from: "sender@private.example", label: "INBOX", size: 200},
		{id: "sensitive-msg", from: "sender@example.com", label: "Sensitive", size: 300},
	}
	for _, fixture := range fixtures {
		if err := NewThreadRepository(db).Upsert(ctx, Thread{AccountID: account.ID, ID: fixture.id + "-thread", LastMessageDate: &when}); err != nil {
			t.Fatalf("upsert thread: %v", err)
		}
		message := Message{
			AccountID:    account.ID,
			ID:           fixture.id,
			ThreadID:     fixture.id + "-thread",
			FromAddr:     fixture.from,
			Subject:      fixture.id,
			SizeBytes:    fixture.size,
			BodyPlain:    fixture.body,
			InternalDate: &when,
		}
		if fixture.cached {
			message.CachedAt = &when
		}
		if err := NewMessageRepository(db).Upsert(ctx, message); err != nil {
			t.Fatalf("upsert message %s: %v", fixture.id, err)
		}
		if err := NewMessageLabelRepository(db).Upsert(ctx, MessageLabel{AccountID: account.ID, MessageID: fixture.id, LabelID: fixture.label}); err != nil {
			t.Fatalf("upsert label mapping %s: %v", fixture.id, err)
		}
	}
	if err := NewAttachmentRepository(db).Upsert(ctx, Attachment{AccountID: account.ID, MessageID: "ada-msg", PartID: "1", Filename: "ada.pdf", SizeBytes: 50}); err != nil {
		t.Fatalf("upsert attachment: %v", err)
	}
	return account.ID
}
