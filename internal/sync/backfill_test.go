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

func TestBackfillerPagesIncludedLabelsAndBatchesMetadata(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX")

	client := newFakeMessageReader()
	refs := numberedRefs(101, "inbox")
	client.pages["INBOX"] = []gmail.ListMessagesPage{
		{Messages: refs[:60], NextPageToken: "page-2"},
		{Messages: refs[60:]},
	}
	for _, ref := range refs {
		client.metadata[ref.ID] = gmail.Message{
			ID:       ref.ID,
			ThreadID: ref.ThreadID,
			LabelIDs: []string{"INBOX"},
			Headers:  []gmail.MessageHeader{{Name: "From", Value: "ada@example.com"}},
		}
	}

	cfg := config.Default()
	cfg.Sync.BackfillLimitPerLabel = 101
	result, err := NewBackfiller(db, cfg, client).Backfill(ctx, account)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(result.Messages) != 101 {
		t.Fatalf("messages = %d, want 101", len(result.Messages))
	}
	if len(result.BodyQueue) != 101 {
		t.Fatalf("body queue = %d, want full-depth messages queued", len(result.BodyQueue))
	}
	if len(client.listCalls) != 2 {
		t.Fatalf("list calls = %#v, want two pages", client.listCalls)
	}
	if client.listCalls[0].opts.Query != "newer_than:90d" || client.listCalls[1].opts.PageToken != "page-2" {
		t.Fatalf("list calls = %#v, want retention query and second page token", client.listCalls)
	}
	if len(client.batchSizes) != 2 || client.batchSizes[0] != 100 || client.batchSizes[1] != 1 {
		t.Fatalf("batch sizes = %#v, want 100 then 1", client.batchSizes)
	}
}

func TestBackfillerSkipsExcludedPolicyLabels(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX", "Label_1")

	if err := cache.NewSyncPolicyRepository(db).Upsert(ctx, cache.SyncPolicy{
		AccountID:      account.ID,
		LabelID:        "Label_1",
		Include:        false,
		Depth:          string(config.CacheDepthNone),
		AttachmentRule: string(config.AttachmentRuleNone),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert excluded policy: %v", err)
	}

	client := newFakeMessageReader()
	client.pages["INBOX"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "inbox-1", ThreadID: "thread-1"}}}}
	client.pages["Label_1"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "excluded-1", ThreadID: "thread-2"}}}}
	client.metadata["inbox-1"] = gmail.Message{ID: "inbox-1", ThreadID: "thread-1", LabelIDs: []string{"INBOX"}}

	result, err := NewBackfiller(db, config.Default(), client).Backfill(ctx, account)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0].ID != "inbox-1" {
		t.Fatalf("messages = %#v, want only included inbox message", result.Messages)
	}
	if len(client.listCalls) != 1 || client.listCalls[0].labelID != "INBOX" {
		t.Fatalf("list calls = %#v, want only INBOX listed", client.listCalls)
	}
}

func TestBackfillerBackfillLabelOnlyRelistsRequestedLabel(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX", "Label_1")

	client := newFakeMessageReader()
	client.pages["INBOX"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "inbox-1", ThreadID: "thread-inbox"}}}}
	client.pages["Label_1"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "label-1", ThreadID: "thread-label"}}}}
	client.metadata["label-1"] = gmail.Message{
		ID:       "label-1",
		ThreadID: "thread-label",
		LabelIDs: []string{"Label_1"},
		Headers:  []gmail.MessageHeader{{Name: "From", Value: "label@example.com"}},
	}

	result, err := NewBackfiller(db, config.Default(), client).BackfillLabel(ctx, account, "Label_1")
	if err != nil {
		t.Fatalf("backfill label: %v", err)
	}
	if len(result.Messages) != 1 || result.Messages[0].ID != "label-1" {
		t.Fatalf("messages = %#v, want requested label message", result.Messages)
	}
	if len(result.Labels) != 1 || result.Labels[0].LabelID != "Label_1" {
		t.Fatalf("labels = %#v, want only requested label", result.Labels)
	}
	if len(client.listCalls) != 1 || client.listCalls[0].labelID != "Label_1" {
		t.Fatalf("list calls = %#v, want only Label_1 relisted", client.listCalls)
	}
}

func TestBackfillerQueuesBodiesByPolicyDepthAndExclusion(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "SPAM", "SENT", "INBOX", "Label_Secret")

	if err := cache.NewSyncPolicyRepository(db).Upsert(ctx, cache.SyncPolicy{
		AccountID:      account.ID,
		LabelID:        "Label_Secret",
		Include:        true,
		Depth:          string(config.CacheDepthFull),
		AttachmentRule: string(config.AttachmentRuleAll),
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert secret policy: %v", err)
	}
	if err := cache.NewCacheExclusionRepository(db).Upsert(ctx, cache.CacheExclusion{
		AccountID:  account.ID,
		MatchType:  "label",
		MatchValue: "Label_Secret",
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert exclusion: %v", err)
	}

	client := newFakeMessageReader()
	for _, labelID := range []string{"SPAM", "SENT", "INBOX", "Label_Secret"} {
		id := labelID + "-msg"
		client.pages[labelID] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: id, ThreadID: labelID + "-thread"}}}}
		client.metadata[id] = gmail.Message{
			ID:       id,
			ThreadID: labelID + "-thread",
			LabelIDs: []string{labelID},
			Headers:  []gmail.MessageHeader{{Name: "From", Value: "sender@example.com"}},
		}
	}

	result, err := NewBackfiller(db, config.Default(), client).Backfill(ctx, account)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("messages = %#v, want metadata/body/full non-excluded messages", result.Messages)
	}
	if result.ExcludedCount != 1 {
		t.Fatalf("excluded count = %d, want 1", result.ExcludedCount)
	}
	gotQueue := bodyQueueIDs(result.BodyQueue)
	wantQueue := []string{"INBOX-msg", "SENT-msg"}
	if !equalStringSlices(gotQueue, wantQueue) {
		t.Fatalf("body queue = %#v, want %#v", gotQueue, wantQueue)
	}
	if _, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "Label_Secret-msg"); err == nil {
		t.Fatalf("excluded message was persisted")
	} else if !errors.Is(err, cache.ErrMessageNotFound) {
		t.Fatalf("excluded message lookup error = %v, want ErrMessageNotFound", err)
	}
}

func TestBackfillerSkipsSenderExclusionWithoutPersistingBody(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX")
	if err := cache.NewCacheExclusionRepository(db).Upsert(ctx, cache.CacheExclusion{
		AccountID:  account.ID,
		MatchType:  "sender",
		MatchValue: "sensitive@example.com",
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert sender exclusion: %v", err)
	}

	client := newFakeMessageReader()
	client.pages["INBOX"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "sensitive-msg", ThreadID: "thread-sensitive"}}}}
	client.metadata["sensitive-msg"] = gmail.Message{
		ID:       "sensitive-msg",
		ThreadID: "thread-sensitive",
		LabelIDs: []string{"INBOX"},
		Headers:  []gmail.MessageHeader{{Name: "From", Value: "Sensitive <sensitive@example.com>"}},
	}

	result, err := NewBackfiller(db, config.Default(), client).Backfill(ctx, account)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if result.ExcludedCount != 1 || len(result.BodyQueue) != 0 || len(result.Messages) != 0 {
		t.Fatalf("result = %#v, want excluded message with no body queue or persisted metadata", result)
	}
	if _, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "sensitive-msg"); !errors.Is(err, cache.ErrMessageNotFound) {
		t.Fatalf("excluded message lookup error = %v, want ErrMessageNotFound", err)
	}
}

func TestBackfillerPersistsParsedGmailMetadata(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX", "Label_1")

	client := newFakeMessageReader()
	client.pages["INBOX"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "message-1", ThreadID: "thread-1"}}}}
	client.pages["Label_1"] = []gmail.ListMessagesPage{{Messages: []gmail.MessageRef{{ID: "message-1", ThreadID: "thread-1"}}}}
	internalDate := time.Date(2026, 4, 24, 15, 4, 5, 0, time.UTC)
	client.metadata["message-1"] = gmail.Message{
		ID:           "message-1",
		ThreadID:     "thread-1",
		HistoryID:    "88",
		LabelIDs:     []string{"INBOX", "Label_1"},
		Snippet:      "cached snippet",
		SizeEstimate: 2048,
		InternalDate: internalDate,
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "Ada Lovelace <ada@example.com>"},
			{Name: "To", Value: "me@example.com, team@example.com"},
			{Name: "Cc", Value: "ops@example.com"},
			{Name: "Bcc", Value: "audit@example.com"},
			{Name: "Subject", Value: "Metadata sync"},
			{Name: "Date", Value: "Fri, 24 Apr 2026 11:04:05 -0400"},
		},
	}

	result, err := NewBackfiller(db, config.Default(), client).Backfill(ctx, account)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("messages = %#v, want one persisted metadata message", result.Messages)
	}

	thread, err := cache.NewThreadRepository(db).Get(ctx, account.ID, "thread-1")
	if err != nil {
		t.Fatalf("get thread: %v", err)
	}
	if thread.Snippet != "cached snippet" || thread.HistoryID != "88" || thread.LastMessageDate == nil || !thread.LastMessageDate.Equal(internalDate) {
		t.Fatalf("thread = %#v, want Gmail thread metadata", thread)
	}

	message, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if message.FromAddr != "Ada Lovelace <ada@example.com>" || message.Subject != "Metadata sync" || message.SizeBytes != 2048 {
		t.Fatalf("message = %#v, want parsed Gmail metadata", message)
	}
	if !equalStringSlices(message.ToAddrs, []string{"me@example.com", "team@example.com"}) {
		t.Fatalf("to addrs = %#v, want parsed address list", message.ToAddrs)
	}
	if message.Date == nil || !message.Date.Equal(internalDate) {
		t.Fatalf("date = %v, want parsed RFC822 date", message.Date)
	}
	if message.InternalDate == nil || !message.InternalDate.Equal(internalDate) {
		t.Fatalf("internal date = %v, want Gmail internal date", message.InternalDate)
	}
	if message.CachedAt != nil {
		t.Fatalf("cached_at = %v, want nil until body is persisted", message.CachedAt)
	}
	if len(message.RawHeaders) != 6 {
		t.Fatalf("raw headers = %#v, want all Gmail headers", message.RawHeaders)
	}

	labels, err := cache.NewMessageLabelRepository(db).ListForMessage(ctx, account.ID, "message-1")
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(labels) != 2 || labels[0].LabelID != "INBOX" || labels[1].LabelID != "Label_1" {
		t.Fatalf("labels = %#v, want INBOX and Label_1 mappings", labels)
	}
	assertSyncFTSMatchCount(t, db, "metadata", 1)
}

func seedSyncLabels(t *testing.T, db *sqlx.DB, accountID string, labelIDs ...string) {
	t.Helper()

	labels := cache.NewLabelRepository(db)
	for _, labelID := range labelIDs {
		labelType := "user"
		if labelID == "INBOX" || labelID == "SENT" || labelID == "SPAM" {
			labelType = "system"
		}
		if err := labels.Upsert(context.Background(), cache.Label{
			AccountID: accountID,
			ID:        labelID,
			Name:      labelID,
			Type:      labelType,
		}); err != nil {
			t.Fatalf("upsert label %s: %v", labelID, err)
		}
	}
}

type fakeMessageReader struct {
	pages      map[string][]gmail.ListMessagesPage
	pageIndex  map[string]int
	metadata   map[string]gmail.Message
	listCalls  []fakeListCall
	batchSizes []int
}

type fakeListCall struct {
	labelID string
	opts    gmail.ListMessagesOptions
}

func newFakeMessageReader() *fakeMessageReader {
	return &fakeMessageReader{
		pages:     map[string][]gmail.ListMessagesPage{},
		pageIndex: map[string]int{},
		metadata:  map[string]gmail.Message{},
	}
}

func (f *fakeMessageReader) ListMessages(ctx context.Context, opts gmail.ListMessagesOptions) (gmail.ListMessagesPage, error) {
	labelID := ""
	if len(opts.LabelIDs) > 0 {
		labelID = opts.LabelIDs[0]
	}
	f.listCalls = append(f.listCalls, fakeListCall{labelID: labelID, opts: opts})
	index := f.pageIndex[labelID]
	f.pageIndex[labelID]++
	if index >= len(f.pages[labelID]) {
		return gmail.ListMessagesPage{}, nil
	}
	return f.pages[labelID][index], nil
}

func (f *fakeMessageReader) ListHistory(ctx context.Context, opts gmail.ListHistoryOptions) (gmail.HistoryPage, error) {
	return gmail.HistoryPage{}, fmt.Errorf("unexpected history list from backfill fake")
}

func (f *fakeMessageReader) GetMessageMetadata(ctx context.Context, id string, headers ...string) (gmail.Message, error) {
	message, ok := f.metadata[id]
	if !ok {
		return gmail.Message{}, fmt.Errorf("missing metadata fixture %s", id)
	}
	return message, nil
}

func (f *fakeMessageReader) GetMessageFull(ctx context.Context, id string) (gmail.Message, error) {
	return f.GetMessageMetadata(ctx, id)
}

func (f *fakeMessageReader) BatchGetMessageMetadata(ctx context.Context, ids []string, headers ...string) ([]gmail.Message, error) {
	f.batchSizes = append(f.batchSizes, len(ids))
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

func (f *fakeMessageReader) GetThread(ctx context.Context, id string, format gmail.MessageFormat, headers ...string) (gmail.Thread, error) {
	return gmail.Thread{}, fmt.Errorf("unexpected thread fetch %s", id)
}

func numberedRefs(count int, prefix string) []gmail.MessageRef {
	refs := make([]gmail.MessageRef, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%s-%03d", prefix, i)
		refs = append(refs, gmail.MessageRef{ID: id, ThreadID: "thread-" + id})
	}
	return refs
}

func bodyQueueIDs(queue []BodyFetchRequest) []string {
	ids := make([]string, 0, len(queue))
	for _, item := range queue {
		ids = append(ids, item.MessageID)
	}
	return ids
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertSyncFTSMatchCount(t *testing.T, db *sqlx.DB, term string, want int) {
	t.Helper()

	var count int
	if err := db.GetContext(context.Background(), &count, "SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH ?", term); err != nil {
		t.Fatalf("count fts matches for %q: %v", term, err)
	}
	if count != want {
		t.Fatalf("fts matches for %q = %d, want %d", term, count, want)
	}
}
