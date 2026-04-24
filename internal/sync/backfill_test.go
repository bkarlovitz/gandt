package sync

import (
	"context"
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
