package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/compose"
)

func TestComposeHeaderFormInitializesNewMessage(t *testing.T) {
	form := NewComposeHeaderForm(ComposeHeaderFormInput{
		Kind:            compose.ComposeKindNew,
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
		SendAsAliases: []compose.Address{
			compose.NewAddress("Me <me@example.com>"),
			compose.NewAddress("alias@example.com"),
		},
		Width: 20,
	})

	if form.Form == nil {
		t.Fatal("expected huh form")
	}
	if form.Width != 40 {
		t.Fatalf("width = %d, want narrow-safe minimum", form.Width)
	}
	if form.From.Email != "me@example.com" || form.Headers.ActiveAccountID != "acct-1" {
		t.Fatalf("form from/account = %#v/%q, want explicit account and send-as", form.From, form.Headers.ActiveAccountID)
	}
	if form.ToInput != "" || form.Subject != "" {
		t.Fatalf("new message prefill to=%q subject=%q, want empty", form.ToInput, form.Subject)
	}
}

func TestComposeHeaderFormPrefillsReplyModes(t *testing.T) {
	original := compose.OriginalMessage{
		From:    compose.NewAddress("sender@example.com"),
		To:      []compose.Address{compose.NewAddress("me@example.com"), compose.NewAddress("other@example.com")},
		Cc:      []compose.Address{compose.NewAddress("cc@example.com")},
		Subject: "Planning",
		Date:    time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	}

	reply := NewComposeHeaderForm(ComposeHeaderFormInput{
		Kind:            compose.ComposeKindReply,
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
		Original:        original,
	})
	if reply.ToInput != "sender@example.com" || reply.Subject != "Re: Planning" {
		t.Fatalf("reply prefill to=%q subject=%q", reply.ToInput, reply.Subject)
	}

	replyAll := NewComposeHeaderForm(ComposeHeaderFormInput{
		Kind:            compose.ComposeKindReplyAll,
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
		Original:        original,
	})
	if !strings.Contains(replyAll.ToInput, "sender@example.com") || !strings.Contains(replyAll.ToInput, "other@example.com") || !strings.Contains(replyAll.ToInput, "cc@example.com") {
		t.Fatalf("reply-all prefill = %q, want sender, original to minus self, and cc", replyAll.ToInput)
	}
	if strings.Contains(replyAll.ToInput, "me@example.com") {
		t.Fatalf("reply-all prefill includes self: %q", replyAll.ToInput)
	}
}

func TestComposeHeaderFormPrefillsForward(t *testing.T) {
	form := NewComposeHeaderForm(ComposeHeaderFormInput{
		Kind:            compose.ComposeKindForward,
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
		Original:        compose.OriginalMessage{Subject: "Planning"},
	})
	if form.ToInput != "" {
		t.Fatalf("forward to = %q, want empty", form.ToInput)
	}
	if form.Subject != "Fwd: Planning" {
		t.Fatalf("forward subject = %q, want Fwd prefix", form.Subject)
	}
}

func TestComposeHeaderFormSubmitValidationAndCancel(t *testing.T) {
	form := NewComposeHeaderForm(ComposeHeaderFormInput{
		Kind:            compose.ComposeKindNew,
		ActiveAccountID: "acct-1",
		AccountEmail:    "me@example.com",
	})
	form.ToInput = "bad address"
	if _, err := form.Submit(); err == nil || !isComposeHeaderValidationError(err) {
		t.Fatalf("submit error = %v, want address validation error", err)
	}
	if form.Status != ComposeHeaderEditing {
		t.Fatalf("status after validation failure = %s, want editing", form.Status)
	}

	form.ToInput = "you@example.com"
	form.Subject = "Hello"
	headers, err := form.Submit()
	if err != nil {
		t.Fatalf("submit valid form: %v", err)
	}
	if form.Status != ComposeHeaderSubmitted {
		t.Fatalf("status = %s, want submitted", form.Status)
	}
	if len(headers.To) != 1 || headers.To[0].Email != "you@example.com" || headers.Subject != "Hello" {
		t.Fatalf("headers = %#v, want submitted values", headers)
	}

	form.Cancel()
	if form.Status != ComposeHeaderCanceled {
		t.Fatalf("status = %s, want canceled", form.Status)
	}
}
