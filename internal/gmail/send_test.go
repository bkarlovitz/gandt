package gmail

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestClientSendMessage(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/gmail/v1/users/me/messages/send" {
			t.Fatalf("request = %s %s, want messages send", r.Method, r.URL.Path)
		}
		var body struct {
			Raw string `json:"raw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Raw != "cmF3LXNlbmQ" {
			t.Fatalf("raw = %q, want base64url raw-send", body.Raw)
		}
		writeJSON(t, w, map[string]any{"id": "sent-1", "threadId": "thread-1"})
	})

	ref, err := client.SendMessage(context.Background(), []byte("raw-send"))
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if ref.ID != "sent-1" || ref.ThreadID != "thread-1" {
		t.Fatalf("ref = %#v", ref)
	}
}
