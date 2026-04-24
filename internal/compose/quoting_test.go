package compose

import (
	"strings"
	"testing"
	"time"
)

func TestReplyQuoteGolden(t *testing.T) {
	original := OriginalMessage{
		From:      NewAddress("Ada <ada@example.com>"),
		Date:      time.Date(2026, 4, 24, 9, 30, 0, 0, time.UTC),
		BodyPlain: "First line\n\nSecond line",
	}

	got := ReplyQuote(original)
	want := strings.Join([]string{
		"On Apr 24, 2026 at 9:30 AM, \"Ada\" <ada@example.com> wrote:",
		"> First line",
		">",
		"> Second line",
	}, "\n")
	if got != want {
		t.Fatalf("reply quote:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplyAllRecipientsRemoveSelf(t *testing.T) {
	context := NewReplyContext(OriginalMessage{
		From: NewAddress("sender@example.com"),
		To:   []Address{NewAddress("me@example.com"), NewAddress("other@example.com")},
		Cc:   []Address{NewAddress("Me <ME@example.com>"), NewAddress("cc@example.com")},
	}, NewAddress("me@example.com"), true)

	got := emails(context.Recipients())
	want := []string{"sender@example.com", "other@example.com", "cc@example.com"}
	if !sameStrings(got, want) {
		t.Fatalf("reply-all recipients = %v, want %v", got, want)
	}
}

func TestForwardQuoteGolden(t *testing.T) {
	original := OriginalMessage{
		From:      NewAddress("Ada <ada@example.com>"),
		To:        []Address{NewAddress("me@example.com")},
		Cc:        []Address{NewAddress("cc@example.com")},
		Subject:   "Planning",
		Date:      time.Date(2026, 4, 24, 9, 30, 0, 0, time.UTC),
		BodyPlain: "Body text",
	}

	got := ForwardQuote(original)
	want := strings.Join([]string{
		"---------- Forwarded message ---------",
		"From: \"Ada\" <ada@example.com>",
		"Date: Apr 24, 2026 at 9:30 AM",
		"To: me@example.com",
		"Cc: cc@example.com",
		"Subject: Planning",
		"",
		"> Body text",
	}, "\n")
	if got != want {
		t.Fatalf("forward quote:\n%s\nwant:\n%s", got, want)
	}
}
