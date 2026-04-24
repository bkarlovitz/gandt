package compose

import (
	"errors"
	"testing"
	"time"
)

func TestHeadersValidation(t *testing.T) {
	valid := Headers{
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
		SendAs:          NewAddress("Me <me@example.com>"),
		To:              []Address{NewAddress("you@example.com")},
		Subject:         "hello",
	}
	if err := valid.ValidateForSend(); err != nil {
		t.Fatalf("valid headers: %v", err)
	}

	missingRecipient := valid
	missingRecipient.To = nil
	if err := missingRecipient.ValidateForSend(); !errors.Is(err, ErrRecipientRequired) {
		t.Fatalf("missing recipient error = %v, want %v", err, ErrRecipientRequired)
	}

	invalid := valid
	invalid.Cc = []Address{{Email: "not an address"}}
	if err := invalid.ValidateForSend(); err == nil {
		t.Fatal("expected invalid cc address")
	}

	draft := valid
	draft.To = nil
	if err := draft.ValidateDraft(); err != nil {
		t.Fatalf("draft without recipient should validate: %v", err)
	}
}

func TestReplyContextRecipientsAndSubject(t *testing.T) {
	original := OriginalMessage{
		MessageID: "msg-1",
		ThreadID:  "thread-1",
		From:      NewAddress("sender@example.com"),
		To:        []Address{NewAddress("me@example.com"), NewAddress("other@example.com")},
		Cc:        []Address{NewAddress("cc@example.com"), NewAddress("Other <other@example.com>")},
		Subject:   "Status",
		Date:      time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
	}

	reply := NewReplyContext(original, NewAddress("me@example.com"), false)
	if got, want := emails(reply.Recipients()), []string{"sender@example.com"}; !sameStrings(got, want) {
		t.Fatalf("reply recipients = %v, want %v", got, want)
	}
	if got, want := reply.Subject(), "Re: Status"; got != want {
		t.Fatalf("reply subject = %q, want %q", got, want)
	}

	replyAll := NewReplyContext(original, NewAddress("me@example.com"), true)
	want := []string{"sender@example.com", "other@example.com", "cc@example.com"}
	if got := emails(replyAll.Recipients()); !sameStrings(got, want) {
		t.Fatalf("reply-all recipients = %v, want %v", got, want)
	}
}

func TestForwardContextSubject(t *testing.T) {
	forward := NewForwardContext(OriginalMessage{Subject: "Trip notes"})
	if got, want := forward.Subject(), "Fwd: Trip notes"; got != want {
		t.Fatalf("forward subject = %q, want %q", got, want)
	}

	alreadyForwarded := NewForwardContext(OriginalMessage{Subject: "Fwd: Trip notes"})
	if got, want := alreadyForwarded.Subject(), "Fwd: Trip notes"; got != want {
		t.Fatalf("forward subject with existing prefix = %q, want %q", got, want)
	}
}

func TestAttachmentMetadata(t *testing.T) {
	attachment := NewAttachment("/tmp/report.pdf", 1024, "application/pdf")
	if got, want := attachment.Filename, "report.pdf"; got != want {
		t.Fatalf("filename = %q, want %q", got, want)
	}
	if err := attachment.ValidateMetadata(); err != nil {
		t.Fatalf("valid attachment metadata: %v", err)
	}

	attachment.SizeBytes = -1
	if err := attachment.ValidateMetadata(); err == nil {
		t.Fatal("expected negative size to fail")
	}
}

func emails(addresses []Address) []string {
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, address.Email)
	}
	return out
}

func sameStrings(a []string, b []string) bool {
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
