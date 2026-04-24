package gmail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"
)

func TestClientProfileAndLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gmail/v1/users/me/profile":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"emailAddress": "me@example.com",
				"historyId":    "12345",
			})
		case "/gmail/v1/users/me/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"labels": []map[string]any{
					{
						"id":             "INBOX",
						"name":           "Inbox",
						"type":           "system",
						"messagesUnread": 2,
						"messagesTotal":  5,
					},
					{
						"id":             "Label_1",
						"name":           "Receipts",
						"type":           "user",
						"messagesUnread": 1,
						"messagesTotal":  3,
						"color": map[string]string{
							"backgroundColor": "#111111",
							"textColor":       "#eeeeee",
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	profile, err := client.Profile(context.Background())
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.EmailAddress != "me@example.com" || profile.HistoryID != "12345" {
		t.Fatalf("profile = %#v, want email/history", profile)
	}

	labels, err := client.Labels(context.Background())
	if err != nil {
		t.Fatalf("labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("labels = %#v, want 2", labels)
	}
	if labels[1].ID != "Label_1" || labels[1].Unread != 1 || labels[1].ColorBG != "#111111" || labels[1].ColorFG != "#eeeeee" {
		t.Fatalf("custom label = %#v, want parsed Gmail label", labels[1])
	}
}
