package gmail

import (
	"context"
	"net/http"
	"testing"
)

func TestClientListSendAs(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gmail/v1/users/me/settings/sendAs" {
			t.Fatalf("path = %s, want send-as list", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{
			"sendAs": []map[string]any{
				{
					"sendAsEmail":        "me@example.com",
					"displayName":        "Me",
					"replyToAddress":     "reply@example.com",
					"isDefault":          true,
					"isPrimary":          true,
					"verificationStatus": "accepted",
				},
				{
					"sendAsEmail":        "alias@example.com",
					"displayName":        "Alias",
					"verificationStatus": "pending",
				},
				{
					"displayName": "missing email",
				},
			},
		})
	})

	identities, err := client.ListSendAs(context.Background())
	if err != nil {
		t.Fatalf("list send-as: %v", err)
	}
	if len(identities) != 2 {
		t.Fatalf("identities = %#v, want two with email addresses", identities)
	}
	if identities[0].Email != "me@example.com" || !identities[0].IsDefault || !identities[0].IsPrimary || identities[0].ReplyToAddress != "reply@example.com" {
		t.Fatalf("primary identity = %#v, want parsed fields", identities[0])
	}
	if identities[1].Email != "alias@example.com" || identities[1].VerificationStatus != "pending" {
		t.Fatalf("alias identity = %#v, want parsed alias", identities[1])
	}
}
