package sync

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestBackfillerFetchBodiesPersistsFullBodyAndAttachmentMetadata(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "INBOX")

	client := newFakeMessageReader()
	client.metadata["message-1"] = gmail.Message{
		ID:           "message-1",
		ThreadID:     "thread-1",
		LabelIDs:     []string{"INBOX"},
		Snippet:      "full body",
		SizeEstimate: 2048,
		Headers: []gmail.MessageHeader{
			{Name: "From", Value: "ada@example.com"},
			{Name: "Subject", Value: "Body sync"},
		},
		Payload: &gmail.MessagePart{
			MimeType: "multipart/mixed",
			Parts: []gmail.MessagePart{
				{PartID: "1", MimeType: "text/plain", Body: gmail.MessagePartBody{Data: gmailBody("plain body")}},
				{PartID: "2", MimeType: "text/html", Body: gmail.MessagePartBody{Data: gmailBody("<p>html body</p>")}},
				{
					PartID:   "3",
					MimeType: "application/pdf",
					Filename: "plan.pdf",
					Body:     gmail.MessagePartBody{Size: 2048, AttachmentID: "att-1"},
				},
			},
		},
	}

	result, err := NewBackfiller(db, config.Default(), client).FetchBodies(ctx, account, []BodyFetchRequest{
		{MessageID: "message-1", ThreadID: "thread-1", Depth: config.CacheDepthFull},
	})
	if err != nil {
		t.Fatalf("fetch bodies: %v", err)
	}
	if result.Requested != 1 || result.Fetched != 1 || result.Cached != 1 {
		t.Fatalf("result = %#v, want one fetched and cached", result)
	}

	message, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if message.BodyPlain == nil || *message.BodyPlain != "plain body" {
		t.Fatalf("body plain = %v, want plain body", message.BodyPlain)
	}
	if message.BodyHTML == nil || *message.BodyHTML != "<p>html body</p>" {
		t.Fatalf("body html = %v, want raw html", message.BodyHTML)
	}
	if !message.FetchedFull || message.CachedAt == nil {
		t.Fatalf("full cache fields = fetchedFull %v cachedAt %v, want full cached body", message.FetchedFull, message.CachedAt)
	}

	attachments, err := cache.NewAttachmentRepository(db).ListForMessage(ctx, account.ID, "message-1")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Filename != "plan.pdf" || attachments[0].AttachmentID != "att-1" {
		t.Fatalf("attachments = %#v, want persisted pdf metadata", attachments)
	}
}

func TestBackfillerFetchBodiesRendersHTMLFallbackForBodyPolicy(t *testing.T) {
	ctx := context.Background()
	db := migratedSyncTestDB(t)
	account := seedSyncAccount(t, db)
	seedSyncLabels(t, db, account.ID, "SENT")

	client := newFakeMessageReader()
	client.metadata["message-1"] = gmail.Message{
		ID:       "message-1",
		ThreadID: "thread-1",
		LabelIDs: []string{"SENT"},
		Headers:  []gmail.MessageHeader{{Name: "From", Value: "me@example.com"}},
		Payload: &gmail.MessagePart{
			PartID:   "1",
			MimeType: "text/html",
			Body:     gmail.MessagePartBody{Data: gmailBody("<p>Hello</p><p>Fallback</p>")},
		},
	}

	_, err := NewBackfiller(db, config.Default(), client).FetchBodies(ctx, account, []BodyFetchRequest{
		{MessageID: "message-1", ThreadID: "thread-1", Depth: config.CacheDepthBody},
	})
	if err != nil {
		t.Fatalf("fetch bodies: %v", err)
	}

	message, err := cache.NewMessageRepository(db).Get(ctx, account.ID, "message-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if message.BodyPlain == nil || *message.BodyPlain != "Hello\n\nFallback" {
		got := "<nil>"
		if message.BodyPlain != nil {
			got = *message.BodyPlain
		}
		t.Fatalf("body plain = %q, want rendered html fallback", got)
	}
	if message.BodyHTML != nil {
		t.Fatalf("body html = %v, want nil for body-depth policy", message.BodyHTML)
	}
	if message.FetchedFull {
		t.Fatalf("fetched_full = true, want false for body-depth policy")
	}
}

func gmailBody(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
