package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/option"
)

func TestClientListMessagesUsesRequestParameters(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages" {
			t.Fatalf("path = %s, want messages list", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query["labelIds"]; !equalStrings(got, []string{"INBOX", "Label_1"}) {
			t.Fatalf("labelIds = %#v, want INBOX and Label_1", got)
		}
		if query.Get("q") != "newer_than:90d" {
			t.Fatalf("q = %q, want newer_than:90d", query.Get("q"))
		}
		if query.Get("pageToken") != "next-token" {
			t.Fatalf("pageToken = %q, want next-token", query.Get("pageToken"))
		}
		if query.Get("maxResults") != "50" {
			t.Fatalf("maxResults = %q, want 50", query.Get("maxResults"))
		}
		if query.Get("includeSpamTrash") != "true" {
			t.Fatalf("includeSpamTrash = %q, want true", query.Get("includeSpamTrash"))
		}

		writeJSON(t, w, map[string]any{
			"messages": []map[string]string{
				{"id": "msg-1", "threadId": "thread-1"},
				{"id": "msg-2", "threadId": "thread-2"},
			},
			"nextPageToken":      "page-2",
			"resultSizeEstimate": 2,
		})
	})

	page, err := client.ListMessages(context.Background(), ListMessagesOptions{
		LabelIDs:         []string{"INBOX", "Label_1"},
		Query:            "newer_than:90d",
		PageToken:        "next-token",
		MaxResults:       50,
		IncludeSpamTrash: true,
	})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(page.Messages) != 2 || page.Messages[0].ID != "msg-1" || page.Messages[0].ThreadID != "thread-1" {
		t.Fatalf("page messages = %#v, want parsed refs", page.Messages)
	}
	if page.NextPageToken != "page-2" || page.ResultSizeEstimate != 2 {
		t.Fatalf("page metadata = %#v, want token and estimate", page)
	}
}

func TestClientGetMessageMetadataParsesHeaders(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages/msg-1" {
			t.Fatalf("path = %s, want message get", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("format") != "metadata" {
			t.Fatalf("format = %q, want metadata", query.Get("format"))
		}
		if got := query["metadataHeaders"]; !equalStrings(got, []string{"From", "Subject"}) {
			t.Fatalf("metadataHeaders = %#v, want From and Subject", got)
		}

		writeJSON(t, w, map[string]any{
			"id":           "msg-1",
			"threadId":     "thread-1",
			"historyId":    "44",
			"labelIds":     []string{"INBOX", "UNREAD"},
			"snippet":      "hello",
			"sizeEstimate": 1234,
			"internalDate": "1777046400000",
			"payload": map[string]any{
				"mimeType": "text/plain",
				"headers": []map[string]string{
					{"name": "From", "value": "Ada <ada@example.com>"},
					{"name": "Subject", "value": "Notes"},
				},
			},
		})
	})

	message, err := client.GetMessageMetadata(context.Background(), "msg-1", "From", "Subject")
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	if message.ID != "msg-1" || message.ThreadID != "thread-1" || message.HistoryID != "44" {
		t.Fatalf("message IDs = %#v, want parsed IDs", message)
	}
	if message.InternalDate != time.UnixMilli(1777046400000).UTC() {
		t.Fatalf("internal date = %s, want parsed Gmail epoch millis", message.InternalDate)
	}
	if len(message.Headers) != 2 || message.Headers[0].Name != "From" || message.Headers[0].Value != "Ada <ada@example.com>" {
		t.Fatalf("headers = %#v, want parsed payload headers", message.Headers)
	}
}

func TestClientGetMessageFullParsesNestedParts(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages/msg-1" {
			t.Fatalf("path = %s, want message get", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "full" {
			t.Fatalf("format = %q, want full", r.URL.Query().Get("format"))
		}

		writeJSON(t, w, map[string]any{
			"id":       "msg-1",
			"threadId": "thread-1",
			"payload": map[string]any{
				"mimeType": "multipart/mixed",
				"parts": []map[string]any{
					{
						"partId":   "0",
						"mimeType": "text/plain",
						"body":     map[string]any{"data": "aGVsbG8", "size": 5},
					},
					{
						"partId":   "1",
						"mimeType": "application/pdf",
						"filename": "one.pdf",
						"body":     map[string]any{"attachmentId": "att-1", "size": 42},
					},
				},
			},
		})
	})

	message, err := client.GetMessageFull(context.Background(), "msg-1")
	if err != nil {
		t.Fatalf("get full: %v", err)
	}
	if message.Payload == nil || len(message.Payload.Parts) != 2 {
		t.Fatalf("payload = %#v, want two parts", message.Payload)
	}
	if message.Payload.Parts[0].Body.Data != "aGVsbG8" {
		t.Fatalf("plain data = %q, want encoded body", message.Payload.Parts[0].Body.Data)
	}
	if message.Payload.Parts[1].Filename != "one.pdf" || message.Payload.Parts[1].Body.AttachmentID != "att-1" {
		t.Fatalf("attachment part = %#v, want filename and attachment id", message.Payload.Parts[1])
	}
}

func TestClientGetThreadParsesMessages(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/threads/thread-1" {
			t.Fatalf("path = %s, want thread get", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "metadata" {
			t.Fatalf("format = %q, want metadata", r.URL.Query().Get("format"))
		}

		writeJSON(t, w, map[string]any{
			"id":        "thread-1",
			"historyId": "99",
			"snippet":   "thread snippet",
			"messages": []map[string]any{
				{"id": "msg-1", "threadId": "thread-1"},
				{"id": "msg-2", "threadId": "thread-1"},
			},
		})
	})

	thread, err := client.GetThread(context.Background(), "thread-1", MessageFormatMetadata)
	if err != nil {
		t.Fatalf("get thread: %v", err)
	}
	if thread.ID != "thread-1" || thread.HistoryID != "99" || len(thread.Messages) != 2 {
		t.Fatalf("thread = %#v, want parsed thread messages", thread)
	}
}

func TestClientNormalizesGmailErrors(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		writeJSON(t, w, map[string]any{
			"error": map[string]any{
				"code":    429,
				"message": "quota exceeded",
				"errors": []map[string]string{
					{"reason": "rateLimitExceeded", "message": "quota exceeded"},
				},
			},
		})
	})
	client.SetRetryPolicy(RetryPolicy{MaxAttempts: 1})

	_, err := client.GetMessageMetadata(context.Background(), "missing")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestClientNormalizesNotFound(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(t, w, map[string]any{
			"error": map[string]any{
				"code":    404,
				"message": "not found",
				"errors": []map[string]string{
					{"reason": "notFound", "message": "not found"},
				},
			},
		})
	})

	_, err := client.GetThread(context.Background(), "missing", MessageFormatFull)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func equalStrings(a, b []string) bool {
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
