package ui

import (
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/auth"
	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestAuthErrorPromptsReauthentication(t *testing.T) {
	model := New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", nil)))

	updated, _ := model.Update(SyncUpdateMsg{Err: gmail.ErrUnauthorized})
	got := updated.(Model)
	if got.statusMessage != "OAuth revoked for me@example.com; re-authenticate the account" || got.toastMessage != got.statusMessage {
		t.Fatalf("status/toast = %q/%q, want re-auth prompt", got.statusMessage, got.toastMessage)
	}
}

func TestNonFatalErrorRendersToast(t *testing.T) {
	model := New(config.Default())

	updated, cmd := model.Update(ErrorMsg{Err: errors.New("network down")})
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected toast dismissal command")
	}
	if got.statusMessage != "network down" || got.toastMessage != "network down" {
		t.Fatalf("status/toast = %q/%q, want non-fatal toast", got.statusMessage, got.toastMessage)
	}
}

func TestToastAutoDismissIgnoresStaleTimers(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(ErrorMsg{Err: errors.New("first")})
	model = updated.(Model)
	firstGeneration := model.toastGeneration

	updated, _ = model.Update(ErrorMsg{Err: errors.New("second")})
	model = updated.(Model)
	secondGeneration := model.toastGeneration
	if secondGeneration == firstGeneration {
		t.Fatalf("toast generation did not advance: %d", secondGeneration)
	}

	updated, _ = model.Update(toastExpiredMsg{Generation: firstGeneration})
	model = updated.(Model)
	if model.toastMessage != "second" {
		t.Fatalf("stale timer cleared toast: %q", model.toastMessage)
	}

	updated, _ = model.Update(toastExpiredMsg{Generation: secondGeneration})
	model = updated.(Model)
	if model.toastMessage != "" || model.statusMessage != "second" {
		t.Fatalf("toast/status = %q/%q, want dismissed toast with status retained", model.toastMessage, model.statusMessage)
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

func TestDistinctServiceErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "keychain", err: auth.ErrKeyringUnavailable, want: "keychain inaccessible; unlock the OS keychain and retry"},
		{name: "rate limited", err: gmail.ErrRateLimited, want: "Gmail rate limited requests; wait and retry"},
		{name: "offline", err: MarkOffline(errors.New("dial tcp failed")), want: "offline: cached mail available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(config.Default(), WithMailbox(RealAccountMailbox("me@example.com", nil)))
			updated, _ := model.Update(ErrorMsg{Err: tt.err})
			got := updated.(Model)
			if got.statusMessage != tt.want || got.toastMessage != tt.want {
				t.Fatalf("status/toast = %q/%q, want %q", got.statusMessage, got.toastMessage, tt.want)
			}
		})
	}
}

func TestCacheCorruptionRendersRecoverableFatalScreen(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(ErrorMsg{Err: cache.ErrCorrupt})
	got := updated.(Model)
	if got.fatalError != "cache database corrupt; quit and inspect or remove cache.db" {
		t.Fatalf("fatal error = %q, want cache corruption guidance", got.fatalError)
	}
	if view := got.View(); view != "G&T\n\nFatal error: cache database corrupt; quit and inspect or remove cache.db" {
		t.Fatalf("view = %q, want cache corruption screen", view)
	}
}
