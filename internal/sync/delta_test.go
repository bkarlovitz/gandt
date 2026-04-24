package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/jmoiron/sqlx"
)

func TestDeltaSyncAppliesHistoryRecordsAndAdvancesHistoryAfterSuccess(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccountWithHistory(t, db, "100")
	seedSyncLabels(t, db, account.ID, "INBOX", "UNREAD", "Label_1")
	seedMetadataPolicy(t, db, account.ID, "INBOX")
	seedMetadataPolicy(t, db, account.ID, "UNREAD")
	seedMetadataPolicy(t, db, account.ID, "Label_1")
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	seedCachedMessage(t, db, account.ID, cache.Message{ID: "keep-1", ThreadID: "thread-keep", Subject: "Keep", InternalDate: &now}, []string{"INBOX"})
	seedCachedMessage(t, db, account.ID, cache.Message{ID: "delete-1", ThreadID: "thread-delete", Subject: "Delete", InternalDate: &now}, []string{"INBOX"})

	client := newFakeHistoryReader()
	client.historyPages = []gmail.HistoryPage{{
		HistoryID: "200",
		Records: []gmail.HistoryRecord{{
			ID:              "101",
			MessagesAdded:   []gmail.HistoryMessageChange{{Message: gmail.MessageRef{ID: "new-1", ThreadID: "thread-new"}}},
			MessagesDeleted: []gmail.HistoryMessageChange{{Message: gmail.MessageRef{ID: "delete-1", ThreadID: "thread-delete"}}},
			LabelsAdded:     []gmail.HistoryLabelChange{{Message: gmail.MessageRef{ID: "keep-1", ThreadID: "thread-keep"}, LabelIDs: []string{"UNREAD"}}},
			LabelsRemoved:   []gmail.HistoryLabelChange{{Message: gmail.MessageRef{ID: "keep-1", ThreadID: "thread-keep"}, LabelIDs: []string{"INBOX"}}},
		}},
	}}
	client.metadata["new-1"] = gmail.Message{
		ID:           "new-1",
		ThreadID:     "thread-new",
		HistoryID:    "199",
		LabelIDs:     []string{"Label_1"},
		Snippet:      "new snippet",
		InternalDate: now.Add(time.Minute),
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "delta@example.com"},
			{Name: "Subject", Value: "Delta new"},
		},
	}

	result, err := NewDeltaSynchronizer(db, config.Default(), client).DeltaSync(ctx, account)
	if err != nil {
		t.Fatalf("delta sync: %v", err)
	}
	if result.HistoryID != "200" || result.MessagesAdded != 1 || result.MessagesDeleted != 1 || result.LabelsAdded != 1 || result.LabelsRemoved != 1 {
		t.Fatalf("result = %#v, want applied history counts", result)
	}
	if len(client.historyCalls) != 1 || client.historyCalls[0].StartHistoryID != "100" {
		t.Fatalf("history calls = %#v, want start history 100", client.historyCalls)
	}

	if _, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "delete-1"); !errors.Is(err, cache.ErrMessageNotFound) {
		t.Fatalf("deleted message error = %v, want ErrMessageNotFound", err)
	}
	newMessage, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "new-1")
	if err != nil {
		t.Fatalf("get new message: %v", err)
	}
	if newMessage.Subject != "Delta new" || newMessage.ThreadID != "thread-new" {
		t.Fatalf("new message = %#v, want persisted metadata", newMessage)
	}
	labels, err := cache.NewMessageLabelRepository(db).ListForMessage(ctx, account.ID, "keep-1")
	if err != nil {
		t.Fatalf("list keep labels: %v", err)
	}
	if len(labels) != 1 || labels[0].LabelID != "UNREAD" {
		t.Fatalf("keep labels = %#v, want only UNREAD", labels)
	}
	updatedAccount, err := cache.NewAccountRepository(db).Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if updatedAccount.HistoryID != "200" || updatedAccount.LastSyncAt == nil {
		t.Fatalf("account sync metadata = %#v, want history 200 and timestamp", updatedAccount)
	}
}

func TestDeltaSyncDoesNotAdvanceHistoryWhenApplyFails(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccountWithHistory(t, db, "100")
	originalLastSync := *account.LastSyncAt
	seedSyncLabels(t, db, account.ID, "INBOX")
	seedMetadataPolicy(t, db, account.ID, "INBOX")

	client := newFakeHistoryReader()
	client.historyPages = []gmail.HistoryPage{{
		HistoryID: "200",
		Records: []gmail.HistoryRecord{{
			MessagesAdded: []gmail.HistoryMessageChange{{Message: gmail.MessageRef{ID: "missing", ThreadID: "thread-missing"}}},
		}},
	}}

	if _, err := NewDeltaSynchronizer(db, config.Default(), client).DeltaSync(ctx, account); err == nil {
		t.Fatal("delta sync unexpectedly succeeded")
	}
	updatedAccount, err := cache.NewAccountRepository(db).Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if updatedAccount.HistoryID != "100" || updatedAccount.LastSyncAt == nil || !updatedAccount.LastSyncAt.Equal(originalLastSync) {
		t.Fatalf("account sync metadata = %#v, want unchanged history", updatedAccount)
	}
}

func TestSyncFallsBackOnExpiredHistoryAndPreservesCachedBodies(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccountWithHistory(t, db, "100")
	seedSyncLabels(t, db, account.ID, "INBOX")
	cachedBody := "cached body"
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	seedCachedMessage(t, db, account.ID, cache.Message{
		ID:           "cached-1",
		ThreadID:     "thread-cached",
		Subject:      "Cached",
		BodyPlain:    &cachedBody,
		InternalDate: &now,
	}, []string{"INBOX"})

	client := newFakeHistoryReader()
	client.historyErr = fmt.Errorf("history gone: %w", gmail.ErrHistoryGone)
	client.listPages["INBOX"] = []gmail.ListMessagesPage{{
		Messages: []gmail.MessageRef{
			{ID: "cached-1", ThreadID: "thread-cached"},
			{ID: "new-1", ThreadID: "thread-new"},
		},
	}}
	client.metadata["cached-1"] = gmail.Message{
		ID:           "cached-1",
		ThreadID:     "thread-cached",
		HistoryID:    "300",
		LabelIDs:     []string{"INBOX"},
		InternalDate: now,
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "cached@example.com"},
			{Name: "Subject", Value: "Cached"},
		},
	}
	client.metadata["new-1"] = gmail.Message{
		ID:           "new-1",
		ThreadID:     "thread-new",
		HistoryID:    "301",
		LabelIDs:     []string{"INBOX"},
		InternalDate: now.Add(time.Minute),
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "new@example.com"},
			{Name: "Subject", Value: "New"},
		},
	}
	client.full["new-1"] = gmail.Message{
		ID:           "new-1",
		ThreadID:     "thread-new",
		HistoryID:    "301",
		LabelIDs:     []string{"INBOX"},
		InternalDate: now.Add(time.Minute),
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "new@example.com"},
			{Name: "Subject", Value: "New"},
		},
		Payload: &gmail.MessagePart{
			MimeType: "text/plain",
			Body:     gmail.MessagePartBody{Data: "bmV3IGJvZHk", Size: 8},
		},
	}

	result, err := NewDeltaSynchronizer(db, config.Default(), client).Sync(ctx, account)
	if err != nil {
		t.Fatalf("sync fallback: %v", err)
	}
	if !result.Fallback || result.Status != "Gmail history expired; refreshing mailbox" {
		t.Fatalf("result = %#v, want expired-history fallback status", result)
	}
	if result.Bodies.Requested != 1 || result.Bodies.Fetched != 1 || len(client.fullCalls) != 1 || client.fullCalls[0] != "new-1" {
		t.Fatalf("body fetches = %#v full calls=%#v, want only new body fetched", result.Bodies, client.fullCalls)
	}
	cached, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "cached-1")
	if err != nil {
		t.Fatalf("get cached message: %v", err)
	}
	if cached.BodyPlain == nil || *cached.BodyPlain != "cached body" {
		t.Fatalf("cached body = %#v, want preserved body", cached.BodyPlain)
	}
	added, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "new-1")
	if err != nil {
		t.Fatalf("get new message: %v", err)
	}
	if added.BodyPlain == nil || *added.BodyPlain != "new body" {
		t.Fatalf("new body = %#v, want fetched body", added.BodyPlain)
	}
	updatedAccount, err := cache.NewAccountRepository(db).Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if updatedAccount.HistoryID != "301" {
		t.Fatalf("account history = %q, want latest backfill message history", updatedAccount.HistoryID)
	}
}

func seedSyncAccountWithHistory(t *testing.T, db *sqlx.DB, historyID string) cache.Account {
	t.Helper()

	account := seedSyncAccount(t, db)
	if err := cache.NewAccountRepository(db).UpdateSyncMetadata(context.Background(), account.ID, historyID, time.Now().UTC()); err != nil {
		t.Fatalf("seed account history: %v", err)
	}
	updated, err := cache.NewAccountRepository(db).Get(context.Background(), account.ID)
	if err != nil {
		t.Fatalf("get seeded account history: %v", err)
	}
	return updated
}

func seedMetadataPolicy(t *testing.T, db *sqlx.DB, accountID string, labelID string) {
	t.Helper()

	if err := cache.NewSyncPolicyRepository(db).Upsert(context.Background(), cache.SyncPolicy{
		AccountID:      accountID,
		LabelID:        labelID,
		Include:        true,
		Depth:          string(config.CacheDepthMetadata),
		AttachmentRule: string(config.AttachmentRuleNone),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert metadata policy %s: %v", labelID, err)
	}
}

func seedCachedMessage(t *testing.T, db *sqlx.DB, accountID string, message cache.Message, labelIDs []string) {
	t.Helper()

	if message.AccountID == "" {
		message.AccountID = accountID
	}
	if err := cache.NewThreadRepository(db).Upsert(context.Background(), cache.Thread{
		AccountID:       accountID,
		ID:              message.ThreadID,
		LastMessageDate: message.InternalDate,
	}); err != nil {
		t.Fatalf("upsert thread %s: %v", message.ThreadID, err)
	}
	if err := cache.NewMessageRepository(db).Upsert(context.Background(), message); err != nil {
		t.Fatalf("upsert message %s: %v", message.ID, err)
	}
	if err := cache.NewMessageLabelRepository(db).ReplaceForMessage(context.Background(), accountID, message.ID, labelIDs); err != nil {
		t.Fatalf("replace labels %s: %v", message.ID, err)
	}
}

type fakeHistoryReader struct {
	historyPages      []gmail.HistoryPage
	historyPageIndex  int
	historyErr        error
	listPages         map[string][]gmail.ListMessagesPage
	listPageIndex     map[string]int
	metadata          map[string]gmail.Message
	full              map[string]gmail.Message
	historyCalls      []gmail.ListHistoryOptions
	fullCalls         []string
	batchMetadataSize []int
}

func newFakeHistoryReader() *fakeHistoryReader {
	return &fakeHistoryReader{
		listPages:     map[string][]gmail.ListMessagesPage{},
		listPageIndex: map[string]int{},
		metadata:      map[string]gmail.Message{},
		full:          map[string]gmail.Message{},
	}
}

func (f *fakeHistoryReader) ListMessages(ctx context.Context, opts gmail.ListMessagesOptions) (gmail.ListMessagesPage, error) {
	labelID := ""
	if len(opts.LabelIDs) > 0 {
		labelID = opts.LabelIDs[0]
	}
	index := f.listPageIndex[labelID]
	f.listPageIndex[labelID]++
	if index >= len(f.listPages[labelID]) {
		return gmail.ListMessagesPage{}, nil
	}
	return f.listPages[labelID][index], nil
}

func (f *fakeHistoryReader) ListHistory(ctx context.Context, opts gmail.ListHistoryOptions) (gmail.HistoryPage, error) {
	f.historyCalls = append(f.historyCalls, opts)
	if f.historyErr != nil {
		return gmail.HistoryPage{}, f.historyErr
	}
	if f.historyPageIndex >= len(f.historyPages) {
		return gmail.HistoryPage{}, nil
	}
	page := f.historyPages[f.historyPageIndex]
	f.historyPageIndex++
	return page, nil
}

func (f *fakeHistoryReader) GetMessageMetadata(ctx context.Context, id string, headers ...string) (gmail.Message, error) {
	message, ok := f.metadata[id]
	if !ok {
		return gmail.Message{}, fmt.Errorf("missing metadata fixture %s", id)
	}
	return message, nil
}

func (f *fakeHistoryReader) GetMessageFull(ctx context.Context, id string) (gmail.Message, error) {
	f.fullCalls = append(f.fullCalls, id)
	if message, ok := f.full[id]; ok {
		return message, nil
	}
	return f.GetMessageMetadata(ctx, id)
}

func (f *fakeHistoryReader) BatchGetMessageMetadata(ctx context.Context, ids []string, headers ...string) ([]gmail.Message, error) {
	f.batchMetadataSize = append(f.batchMetadataSize, len(ids))
	messages := make([]gmail.Message, 0, len(ids))
	for _, id := range ids {
		message, err := f.GetMessageMetadata(ctx, id, headers...)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (f *fakeHistoryReader) GetThread(ctx context.Context, id string, format gmail.MessageFormat, headers ...string) (gmail.Thread, error) {
	return gmail.Thread{}, fmt.Errorf("unexpected thread fetch")
}
