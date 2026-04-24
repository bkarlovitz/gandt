package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestClientBatchModifyMessagesBuildsRequest(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/gmail/v1/users/me/messages/batchModify" {
			t.Fatalf("request = %s %s, want batchModify", r.Method, r.URL.Path)
		}
		var body map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !equalStrings(body["ids"], []string{"msg-1", "msg-2"}) {
			t.Fatalf("ids = %#v, want message IDs", body["ids"])
		}
		if !equalStrings(body["addLabelIds"], []string{"STARRED"}) {
			t.Fatalf("add labels = %#v, want STARRED", body["addLabelIds"])
		}
		if !equalStrings(body["removeLabelIds"], []string{"UNREAD"}) {
			t.Fatalf("remove labels = %#v, want UNREAD", body["removeLabelIds"])
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.BatchModifyMessages(context.Background(), MessageModifyRequest{
		IDs:            []string{"msg-1", "msg-2"},
		AddLabelIDs:    []string{"STARRED"},
		RemoveLabelIDs: []string{"UNREAD"},
	}); err != nil {
		t.Fatalf("batch modify: %v", err)
	}
}

func TestClientTrashAndUntrashMessagesUseActionEndpoints(t *testing.T) {
	paths := []string{}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writeJSON(t, w, map[string]any{"id": "msg-1"})
	})

	if err := client.TrashMessage(context.Background(), "msg-1"); err != nil {
		t.Fatalf("trash: %v", err)
	}
	if err := client.UntrashMessage(context.Background(), "msg-1"); err != nil {
		t.Fatalf("untrash: %v", err)
	}
	want := []string{
		"/gmail/v1/users/me/messages/msg-1/trash",
		"/gmail/v1/users/me/messages/msg-1/untrash",
	}
	if !equalStrings(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestClientUntrashMessageSupportsTrashUndo(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/gmail/v1/users/me/messages/msg-1/untrash" {
			t.Fatalf("request = %s %s, want untrash undo endpoint", r.Method, r.URL.Path)
		}
		writeJSON(t, w, map[string]any{"id": "msg-1"})
	})

	if err := client.UntrashMessage(context.Background(), "msg-1"); err != nil {
		t.Fatalf("untrash undo: %v", err)
	}
}

func TestClientModifyThreadBuildsRequest(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/gmail/v1/users/me/threads/thread-1/modify" {
			t.Fatalf("request = %s %s, want thread modify", r.Method, r.URL.Path)
		}
		var body map[string][]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !equalStrings(body["addLabelIds"], []string{"MUTED"}) || !equalStrings(body["removeLabelIds"], []string{"INBOX"}) {
			t.Fatalf("body = %#v, want MUTED add and INBOX remove", body)
		}
		writeJSON(t, w, map[string]any{"id": "thread-1"})
	})

	if err := client.ModifyThread(context.Background(), ThreadModifyRequest{
		ThreadID:       "thread-1",
		AddLabelIDs:    []string{"MUTED"},
		RemoveLabelIDs: []string{"INBOX"},
	}); err != nil {
		t.Fatalf("modify thread: %v", err)
	}
}

func TestClientCreateAndDeleteLabel(t *testing.T) {
	paths := []string{}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/gmail/v1/users/me/labels":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create label: %v", err)
			}
			if body["name"] != "Receipts" || body["labelListVisibility"] != "labelShow" || body["messageListVisibility"] != "show" {
				t.Fatalf("create body = %#v, want label visibility settings", body)
			}
			writeJSON(t, w, map[string]any{
				"id":             "Label_1",
				"name":           "Receipts",
				"type":           "user",
				"messagesUnread": 3,
			})
		case "/gmail/v1/users/me/labels/Label_1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	label, err := client.CreateLabel(context.Background(), LabelCreateRequest{
		Name:                  "Receipts",
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	})
	if err != nil {
		t.Fatalf("create label: %v", err)
	}
	if label.ID != "Label_1" || label.Name != "Receipts" || label.Unread != 3 {
		t.Fatalf("label = %#v, want parsed label", label)
	}
	if err := client.DeleteLabel(context.Background(), "Label_1"); err != nil {
		t.Fatalf("delete label: %v", err)
	}
	want := []string{"POST /gmail/v1/users/me/labels", "DELETE /gmail/v1/users/me/labels/Label_1"}
	if !equalStrings(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestActionWrappersNormalizeAuthAndPermissionErrors(t *testing.T) {
	tests := []struct {
		name string
		code int
		want error
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, want: ErrUnauthorized},
		{name: "forbidden", code: http.StatusForbidden, want: ErrForbidden},
		{name: "not found", code: http.StatusNotFound, want: ErrNotFound},
		{name: "rate limited", code: http.StatusTooManyRequests, want: ErrRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				writeJSON(t, w, map[string]any{
					"error": map[string]any{
						"code":    tt.code,
						"message": tt.name,
						"errors": []map[string]string{
							{"reason": tt.name, "message": tt.name},
						},
					},
				})
			})
			err := client.TrashMessage(context.Background(), "msg-1")
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
