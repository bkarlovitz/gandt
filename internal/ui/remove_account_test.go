package ui

import (
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRemoveAccountRequiresConfirmationAndClearsActiveAccount(t *testing.T) {
	var removed string
	model := New(config.Default(), WithAccountRemover(AccountRemoverFunc(func(account string) (AccountRemoveResult, error) {
		removed = account
		return AccountRemoveResult{Account: account}, nil
	})))
	model.mailbox = RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}})

	updated, cmd := submitTestCommand(model, "remove-account")
	got := updated.(Model)
	if cmd != nil || got.pendingRemoveAccount != "me@example.com" {
		t.Fatalf("cmd/pending = %T/%q, want confirmation", cmd, got.pendingRemoveAccount)
	}
	if got.statusMessage != "remove account me@example.com? y confirm / n cancel" {
		t.Fatalf("status = %q, want confirmation", got.statusMessage)
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got = updated.(Model)
	if cmd == nil || !got.removingAccount {
		t.Fatalf("cmd/removing = %T/%v, want remove command", cmd, got.removingAccount)
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if removed != "me@example.com" || got.mailbox.Account != "no accounts" || !got.mailbox.NoAccounts {
		t.Fatalf("removed/mailbox = %q/%#v, want active account removed", removed, got.mailbox)
	}
	if got.statusMessage != "removed account me@example.com" {
		t.Fatalf("status = %q, want removed status", got.statusMessage)
	}
}

func TestRemoveAccountCanCancel(t *testing.T) {
	calls := 0
	model := New(config.Default(), WithAccountRemover(AccountRemoverFunc(func(account string) (AccountRemoveResult, error) {
		calls++
		return AccountRemoveResult{}, nil
	})))
	model.mailbox = RealAccountMailbox("me@example.com", nil)

	updated, _ := submitTestCommand(model, "remove-account")
	updated, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(Model)
	if cmd != nil || calls != 0 || got.pendingRemoveAccount != "" {
		t.Fatalf("cmd/calls/pending = %T/%d/%q, want canceled", cmd, calls, got.pendingRemoveAccount)
	}
	if got.statusMessage != "remove account canceled" {
		t.Fatalf("status = %q, want canceled", got.statusMessage)
	}
}

func TestRemoveAccountReportsRevokeFailureAndErrors(t *testing.T) {
	model := New(config.Default(), WithAccountRemover(AccountRemoverFunc(func(account string) (AccountRemoveResult, error) {
		return AccountRemoveResult{Account: account, RevokeError: true}, nil
	})))
	model.mailbox = RealAccountMailbox("me@example.com", nil)
	updated, _ := submitTestCommand(model, "remove-account")
	updated, cmd := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated, _ = updated.(Model).Update(cmd())
	if got := updated.(Model); got.statusMessage != "removed account me@example.com; token revoke failed" {
		t.Fatalf("status = %q, want revoke failure noted", got.statusMessage)
	}

	model = New(config.Default(), WithAccountRemover(AccountRemoverFunc(func(account string) (AccountRemoveResult, error) {
		return AccountRemoveResult{}, errors.New("keyring failed")
	})))
	model.mailbox = RealAccountMailbox("me@example.com", nil)
	updated, _ = submitTestCommand(model, "remove-account")
	updated, cmd = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated, _ = updated.(Model).Update(cmd())
	if got := updated.(Model); got.statusMessage != "remove account failed: keyring failed" {
		t.Fatalf("status = %q, want remove error", got.statusMessage)
	}
}
