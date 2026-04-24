package ui

import "testing"

func TestReopenDraftMessageBuildsComposeState(t *testing.T) {
	state := ReopenDraftMessage("acct-1", "me@example.com", Message{
		ID:       "draft-1",
		ThreadID: "thread-1",
		Subject:  "Draft subject",
		Body:     []string{"line one", "line two"},
	})

	if state.DraftID.GmailDraftID != "draft-1" || state.DraftID.ThreadID != "thread-1" {
		t.Fatalf("draft id = %#v", state.DraftID)
	}
	if state.Headers.ActiveAccountID != "acct-1" || state.Headers.SendAs.Email != "me@example.com" || state.Headers.Subject != "Draft subject" {
		t.Fatalf("headers = %#v", state.Headers)
	}
	if state.Body != "line one\nline two" {
		t.Fatalf("body = %q", state.Body)
	}
}
