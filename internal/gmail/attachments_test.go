package gmail

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"
)

func TestClientGetAttachment(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/messages/msg-1/attachments/att-1" {
			t.Fatalf("path = %s, want attachment get", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{
			"data": base64.RawURLEncoding.EncodeToString([]byte("file bytes")),
			"size": 10,
		})
	})

	body, err := client.GetAttachment(context.Background(), "msg-1", "att-1")
	if err != nil {
		t.Fatalf("get attachment: %v", err)
	}
	if string(body) != "file bytes" {
		t.Fatalf("body = %q", string(body))
	}
}
