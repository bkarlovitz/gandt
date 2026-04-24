package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestThreadMessageLabelAndAttachmentRepositories(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accountID := seedMessageRepoAccount(t, db)

	threads := NewThreadRepository(db)
	messages := NewMessageRepository(db)
	messageLabels := NewMessageLabelRepository(db)
	attachments := NewAttachmentRepository(db)

	lastMessageDate := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	thread := Thread{
		AccountID:       accountID,
		ID:              "thread-1",
		Snippet:         "thread snippet",
		HistoryID:       "100",
		LastMessageDate: &lastMessageDate,
	}
	if err := threads.Upsert(ctx, thread); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	thread.Snippet = "updated snippet"
	thread.HistoryID = "101"
	if err := threads.Upsert(ctx, thread); err != nil {
		t.Fatalf("upsert updated thread: %v", err)
	}
	gotThread, err := threads.Get(ctx, accountID, "thread-1")
	if err != nil {
		t.Fatalf("get thread: %v", err)
	}
	if gotThread.Snippet != "updated snippet" || gotThread.HistoryID != "101" || !gotThread.LastMessageDate.Equal(lastMessageDate) {
		t.Fatalf("thread = %#v, want updated thread", gotThread)
	}

	messageDate := time.Date(2026, 4, 24, 11, 30, 0, 0, time.UTC)
	cachedAt := time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)
	plainBody := "original quarterly body"
	htmlBody := "<p>original quarterly body</p>"
	message := Message{
		AccountID:    accountID,
		ID:           "message-1",
		ThreadID:     "thread-1",
		FromAddr:     "ada@example.com",
		ToAddrs:      []string{"me@example.com", "team@example.com"},
		CcAddrs:      []string{"ops@example.com"},
		BccAddrs:     []string{"audit@example.com"},
		Subject:      "Quarterly plan",
		Date:         &messageDate,
		Snippet:      "planning",
		SizeBytes:    4096,
		BodyPlain:    &plainBody,
		BodyHTML:     &htmlBody,
		RawHeaders:   []Header{{Name: "From", Value: "Ada <ada@example.com>"}, {Name: "Subject", Value: "Quarterly plan"}},
		InternalDate: &messageDate,
		FetchedFull:  true,
		CachedAt:     &cachedAt,
	}
	if err := messages.Upsert(ctx, message); err != nil {
		t.Fatalf("upsert message: %v", err)
	}
	updatedBody := "updated release body"
	message.Subject = "Release plan"
	message.BodyPlain = &updatedBody
	if err := messages.Upsert(ctx, message); err != nil {
		t.Fatalf("upsert updated message: %v", err)
	}

	gotMessage, err := messages.Get(ctx, accountID, "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if gotMessage.Subject != "Release plan" || gotMessage.BodyPlain == nil || *gotMessage.BodyPlain != "updated release body" {
		t.Fatalf("message = %#v, want updated body and subject", gotMessage)
	}
	if !equalStrings(gotMessage.ToAddrs, []string{"me@example.com", "team@example.com"}) || !equalStrings(gotMessage.CcAddrs, []string{"ops@example.com"}) {
		t.Fatalf("message recipients = to %#v cc %#v, want decoded JSON arrays", gotMessage.ToAddrs, gotMessage.CcAddrs)
	}
	if len(gotMessage.RawHeaders) != 2 || gotMessage.RawHeaders[0].Name != "From" {
		t.Fatalf("raw headers = %#v, want decoded typed headers", gotMessage.RawHeaders)
	}
	assertStoredJSON(t, db, "to_addrs", accountID, "message-1")
	assertStoredJSON(t, db, "raw_headers", accountID, "message-1")

	if err := messageLabels.Upsert(ctx, MessageLabel{AccountID: accountID, MessageID: "message-1", LabelID: "INBOX"}); err != nil {
		t.Fatalf("upsert inbox label: %v", err)
	}
	if err := messageLabels.Upsert(ctx, MessageLabel{AccountID: accountID, MessageID: "message-1", LabelID: "INBOX"}); err != nil {
		t.Fatalf("upsert duplicate label: %v", err)
	}
	if err := messageLabels.Upsert(ctx, MessageLabel{AccountID: accountID, MessageID: "message-1", LabelID: "STARRED"}); err != nil {
		t.Fatalf("upsert starred label: %v", err)
	}
	labels, err := messageLabels.ListForMessage(ctx, accountID, "message-1")
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(labels) != 2 || labels[0].LabelID != "INBOX" || labels[1].LabelID != "STARRED" {
		t.Fatalf("labels = %#v, want sorted unique mappings", labels)
	}
	if _, err := messageLabels.Get(ctx, accountID, "message-1", "INBOX"); err != nil {
		t.Fatalf("get label mapping: %v", err)
	}

	labelMessages, err := messages.ListByLabel(ctx, accountID, "INBOX", 10)
	if err != nil {
		t.Fatalf("list by label: %v", err)
	}
	if len(labelMessages) != 1 || labelMessages[0].ID != "message-1" {
		t.Fatalf("label messages = %#v, want message-1", labelMessages)
	}
	threadMessages, err := messages.ListByThread(ctx, accountID, "thread-1")
	if err != nil {
		t.Fatalf("list by thread: %v", err)
	}
	if len(threadMessages) != 1 || threadMessages[0].ID != "message-1" {
		t.Fatalf("thread messages = %#v, want message-1", threadMessages)
	}

	attachment := Attachment{
		AccountID:    accountID,
		MessageID:    "message-1",
		PartID:       "1",
		Filename:     "plan.pdf",
		MimeType:     "application/pdf",
		SizeBytes:    8192,
		AttachmentID: "att-1",
	}
	if err := attachments.Upsert(ctx, attachment); err != nil {
		t.Fatalf("upsert attachment: %v", err)
	}
	attachment.LocalPath = "attachments/plan.pdf"
	if err := attachments.Upsert(ctx, attachment); err != nil {
		t.Fatalf("upsert updated attachment: %v", err)
	}
	gotAttachment, err := attachments.Get(ctx, accountID, "message-1", "1")
	if err != nil {
		t.Fatalf("get attachment: %v", err)
	}
	if gotAttachment.LocalPath != "attachments/plan.pdf" || gotAttachment.AttachmentID != "att-1" {
		t.Fatalf("attachment = %#v, want updated local path", gotAttachment)
	}
	listedAttachments, err := attachments.ListForMessage(ctx, accountID, "message-1")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(listedAttachments) != 1 || listedAttachments[0].Filename != "plan.pdf" {
		t.Fatalf("attachments = %#v, want plan.pdf", listedAttachments)
	}
}

func TestMessageLabelReplaceAndMissingRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accountID := seedMessageRepoAccount(t, db)
	if err := seedThreadAndMessage(ctx, db, accountID); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	messageLabels := NewMessageLabelRepository(db)
	if err := messageLabels.ReplaceForMessage(ctx, accountID, "message-1", []string{"INBOX", "STARRED"}); err != nil {
		t.Fatalf("replace labels: %v", err)
	}
	if err := messageLabels.ReplaceForMessage(ctx, accountID, "message-1", []string{"STARRED"}); err != nil {
		t.Fatalf("replace labels again: %v", err)
	}
	labels, err := messageLabels.ListForMessage(ctx, accountID, "message-1")
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(labels) != 1 || labels[0].LabelID != "STARRED" {
		t.Fatalf("labels = %#v, want only STARRED", labels)
	}
	if _, err := messageLabels.Get(ctx, accountID, "message-1", "INBOX"); !errors.Is(err, ErrMessageLabelAbsent) {
		t.Fatalf("missing mapping error = %v, want ErrMessageLabelAbsent", err)
	}

	if _, err := NewThreadRepository(db).Get(ctx, accountID, "missing"); !errors.Is(err, ErrThreadNotFound) {
		t.Fatalf("missing thread error = %v, want ErrThreadNotFound", err)
	}
	if _, err := NewMessageRepository(db).Get(ctx, accountID, "missing"); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("missing message error = %v, want ErrMessageNotFound", err)
	}
	if _, err := NewAttachmentRepository(db).Get(ctx, accountID, "message-1", "missing"); !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("missing attachment error = %v, want ErrAttachmentNotFound", err)
	}
}

func TestMessageRepositoryCascadesAndFTSTriggers(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accountID := seedMessageRepoAccount(t, db)
	if err := seedThreadAndMessage(ctx, db, accountID); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	assertFTSMatchCount(t, db, "quarterly", 1)

	updated := "updated release notes"
	message, err := NewMessageRepository(db).Get(ctx, accountID, "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	message.Subject = "Release notes"
	message.BodyPlain = &updated
	if err := NewMessageRepository(db).Upsert(ctx, message); err != nil {
		t.Fatalf("update message: %v", err)
	}
	assertFTSMatchCount(t, db, "quarterly", 0)
	assertFTSMatchCount(t, db, "release", 1)

	if err := NewMessageLabelRepository(db).Upsert(ctx, MessageLabel{AccountID: accountID, MessageID: "message-1", LabelID: "INBOX"}); err != nil {
		t.Fatalf("upsert label: %v", err)
	}
	if err := NewAttachmentRepository(db).Upsert(ctx, Attachment{AccountID: accountID, MessageID: "message-1", PartID: "1", Filename: "one.pdf"}); err != nil {
		t.Fatalf("upsert attachment: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM messages WHERE account_id = ? AND id = ?", accountID, "message-1"); err != nil {
		t.Fatalf("delete message: %v", err)
	}

	assertFTSMatchCount(t, db, "release", 0)
	assertRowCount(t, db, "message_labels", accountID, 0)
	assertRowCount(t, db, "attachments", accountID, 0)
}

func seedMessageRepoAccount(t *testing.T, db *sqlx.DB) string {
	t.Helper()

	ctx := context.Background()
	account, err := NewAccountRepository(db).Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	labels := NewLabelRepository(db)
	for _, label := range []Label{
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "STARRED", Name: "Starred", Type: "system"},
	} {
		if err := labels.Upsert(ctx, label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}
	return account.ID
}

func seedThreadAndMessage(ctx context.Context, db *sqlx.DB, accountID string) error {
	messageDate := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if err := NewThreadRepository(db).Upsert(ctx, Thread{AccountID: accountID, ID: "thread-1", LastMessageDate: &messageDate}); err != nil {
		return err
	}
	body := "quarterly plan body"
	return NewMessageRepository(db).Upsert(ctx, Message{
		AccountID:    accountID,
		ID:           "message-1",
		ThreadID:     "thread-1",
		FromAddr:     "ada@example.com",
		ToAddrs:      []string{"me@example.com"},
		Subject:      "Quarterly plan",
		BodyPlain:    &body,
		InternalDate: &messageDate,
	})
}

func assertStoredJSON(t *testing.T, db queryer, column string, accountID string, messageID string) {
	t.Helper()

	var raw string
	if err := db.GetContext(context.Background(), &raw, "SELECT "+column+" FROM messages WHERE account_id = ? AND id = ?", accountID, messageID); err != nil {
		t.Fatalf("read %s: %v", column, err)
	}
	if !json.Valid([]byte(raw)) {
		t.Fatalf("%s = %q, want valid JSON", column, raw)
	}
}

func assertRowCount(t *testing.T, db queryer, table string, accountID string, want int) {
	t.Helper()

	var count int
	if err := db.GetContext(context.Background(), &count, "SELECT COUNT(*) FROM "+table+" WHERE account_id = ?", accountID); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s rows = %d, want %d", table, count, want)
	}
}
