package ui

import (
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestAuthErrorPromptsReauthentication(t *testing.T) {
	model := New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", nil)))

	updated, _ := model.Update(SyncUpdateMsg{Err: gmail.ErrUnauthorized})
	got := updated.(Model)
	if got.statusMessage != "re-authenticate me@example.com" || got.toastMessage != got.statusMessage {
		t.Fatalf("status/toast = %q/%q, want re-auth prompt", got.statusMessage, got.toastMessage)
	}
}

func TestNonFatalErrorRendersToast(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(ErrorMsg{Err: errors.New("network down")})
	got := updated.(Model)
	if got.statusMessage != "network down" || got.toastMessage != "network down" {
		t.Fatalf("status/toast = %q/%q, want non-fatal toast", got.statusMessage, got.toastMessage)
	}
}

func TestFatalErrorRendersClearErrorScreen(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(ErrorMsg{Err: MarkFatal(errors.New("cache open failed"))})
	got := updated.(Model)
	if got.fatalError != "cache open failed" {
		t.Fatalf("fatal error = %q, want stored fatal error", got.fatalError)
	}
	view := got.View()
	if view != "G&T\n\nFatal error: cache open failed" {
		t.Fatalf("view = %q, want fatal error screen", view)
	}
}
