package gmail

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestClientDraftLifecycleWrappers(t *testing.T) {
	seen := map[string]bool{}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path] = true
		switch r.Method + " " + r.URL.Path {
		case "GET /gmail/v1/users/me/drafts":
			if r.URL.Query().Get("q") != "in:drafts" || r.URL.Query().Get("maxResults") != "10" || r.URL.Query().Get("pageToken") != "next" {
				t.Fatalf("draft list query = %s", r.URL.RawQuery)
			}
			writeJSON(t, w, map[string]any{
				"drafts":             []map[string]any{{"id": "draft-1", "message": map[string]any{"id": "msg-1", "threadId": "thread-1"}}},
				"nextPageToken":      "page-2",
				"resultSizeEstimate": 1,
			})
		case "GET /gmail/v1/users/me/drafts/draft-1":
			if r.URL.Query().Get("format") != "raw" {
				t.Fatalf("draft get format = %q", r.URL.Query().Get("format"))
			}
			writeJSON(t, w, map[string]any{
				"id":      "draft-1",
				"message": map[string]any{"id": "msg-1", "threadId": "thread-1", "raw": "cmF3"},
			})
		case "POST /gmail/v1/users/me/drafts":
			assertRawDraft(t, r, "cmF3LWNyZWF0ZQ")
			writeJSON(t, w, map[string]any{"id": "draft-created", "message": map[string]any{"id": "msg-created"}})
		case "PUT /gmail/v1/users/me/drafts/draft-1":
			assertRawDraft(t, r, "cmF3LXVwZGF0ZQ")
			writeJSON(t, w, map[string]any{"id": "draft-1", "message": map[string]any{"id": "msg-updated"}})
		case "DELETE /gmail/v1/users/me/drafts/draft-1":
			w.WriteHeader(http.StatusNoContent)
		case "POST /gmail/v1/users/me/drafts/send":
			writeJSON(t, w, map[string]any{"id": "sent-1", "threadId": "thread-1"})
		default:
			t.Fatalf("unexpected draft request %s %s", r.Method, r.URL.Path)
		}
	})

	page, err := client.ListDrafts(context.Background(), ListDraftsOptions{Query: "in:drafts", PageToken: "next", MaxResults: 10})
	if err != nil || len(page.Drafts) != 1 || page.Drafts[0].ID != "draft-1" || page.NextPageToken != "page-2" {
		t.Fatalf("list drafts = %#v err=%v", page, err)
	}
	ref, message, err := client.GetDraft(context.Background(), "draft-1", "raw")
	if err != nil || ref.ID != "draft-1" || message.Raw != "cmF3" {
		t.Fatalf("get draft ref=%#v message=%#v err=%v", ref, message, err)
	}
	created, err := client.CreateDraft(context.Background(), []byte("raw-create"))
	if err != nil || created.ID != "draft-created" {
		t.Fatalf("create draft = %#v err=%v", created, err)
	}
	updated, err := client.UpdateDraft(context.Background(), "draft-1", []byte("raw-update"))
	if err != nil || updated.Message.ID != "msg-updated" {
		t.Fatalf("update draft = %#v err=%v", updated, err)
	}
	if err := client.DeleteDraft(context.Background(), "draft-1"); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	sent, err := client.SendDraft(context.Background(), "draft-1")
	if err != nil || sent.ID != "sent-1" || sent.ThreadID != "thread-1" {
		t.Fatalf("send draft = %#v err=%v", sent, err)
	}

	for _, key := range []string{
		"GET /gmail/v1/users/me/drafts",
		"GET /gmail/v1/users/me/drafts/draft-1",
		"POST /gmail/v1/users/me/drafts",
		"PUT /gmail/v1/users/me/drafts/draft-1",
		"DELETE /gmail/v1/users/me/drafts/draft-1",
		"POST /gmail/v1/users/me/drafts/send",
	} {
		if !seen[key] {
			t.Fatalf("missing request %s", key)
		}
	}
}

func assertRawDraft(t *testing.T, r *http.Request, want string) {
	t.Helper()
	var body struct {
		Message struct {
			Raw string `json:"raw"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode draft body: %v", err)
	}
	if body.Message.Raw != want {
		t.Fatalf("raw = %q, want %q", body.Message.Raw, want)
	}
}
