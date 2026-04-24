package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/auth"
	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestAddAccountCommandShowsLoadingAndSuccess(t *testing.T) {
	calls := 0
	model := New(config.Default(), WithAccountAdder(AccountAdderFunc(func() (AccountAddResult, error) {
		calls++
		return AccountAddResult{
			Account: "me@example.com",
			Labels: []Label{
				{Name: "Inbox", Unread: 2, System: true},
				{Name: "Receipts", Unread: 1},
			},
		}, nil
	})))

	updated, cmd := submitTestCommand(model, "add-account")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected add-account command")
	}
	if !got.addingAccount || got.statusMessage != "adding account... first run asks for one Desktop OAuth client" {
		t.Fatalf("adding=%v status=%q, want loading state", got.addingAccount, got.statusMessage)
	}

	msg := cmd()
	updated, followup := got.Update(msg)
	got = updated.(Model)
	if followup != nil {
		t.Fatalf("expected no followup command, got %T", followup)
	}
	if calls != 1 {
		t.Fatalf("account adder calls = %d, want 1", calls)
	}
	if got.addingAccount || got.statusMessage != "added account me@example.com" {
		t.Fatalf("adding=%v status=%q, want success state", got.addingAccount, got.statusMessage)
	}
	if got.mailbox.Account != "me@example.com" || len(got.mailbox.Labels) != 2 {
		t.Fatalf("mailbox = %#v, want account labels applied", got.mailbox)
	}
}

func TestAddAccountCommandAppliesEmptyLabelSuccess(t *testing.T) {
	model := New(config.Default(), WithAccountAdder(AccountAdderFunc(func() (AccountAddResult, error) {
		return AccountAddResult{Account: "me@example.com"}, nil
	})))

	updated, cmd := submitTestCommand(model, "add-account")
	if cmd == nil {
		t.Fatal("expected add-account command")
	}

	updated, _ = updated.(Model).Update(cmd())
	got := updated.(Model)
	if !got.mailbox.Real || got.mailbox.Account != "me@example.com" || len(got.mailbox.Labels) != 0 {
		t.Fatalf("mailbox = %#v, want real account with no labels", got.mailbox)
	}
	if strings.Contains(got.View(), "fake inbox") {
		t.Fatalf("view still renders fake inbox after success: %q", got.View())
	}
}

func TestAddAccountCommandCanAddAnotherAccountFromRealMailbox(t *testing.T) {
	model := New(config.Default(), WithAccountAdder(AccountAdderFunc(func() (AccountAddResult, error) {
		return AccountAddResult{
			Account: "second@example.com",
			Labels:  []Label{{ID: "INBOX", Name: "Inbox", Unread: 1, System: true}},
		}, nil
	})))
	model.mailbox = RealAccountMailbox("first@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}}, map[string][]Message{
		"INBOX": {{ID: "first-message", ThreadID: "first-thread", Subject: "First"}},
	})

	updated, cmd := submitTestCommand(model, "add-account")
	if cmd == nil {
		t.Fatal("expected add-account command")
	}
	updated, _ = updated.(Model).Update(cmd())
	got := updated.(Model)
	if got.mailbox.Account != "second@example.com" || len(got.mailbox.Labels) != 1 {
		t.Fatalf("mailbox = %#v, want newly added second account active", got.mailbox)
	}
	if strings.Contains(got.View(), "first-message") {
		t.Fatalf("view leaked previous account message after add: %q", got.View())
	}
}

func TestAddAccountCommandShowsErrorAndKeepsFakeInbox(t *testing.T) {
	model := New(config.Default(), WithAccountAdder(AccountAdderFunc(func() (AccountAddResult, error) {
		return AccountAddResult{}, errors.New("oauth failed")
	})))
	original := model.mailbox

	updated, cmd := submitTestCommand(model, "add-account")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected add-account command")
	}

	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.addingAccount {
		t.Fatal("expected loading to finish")
	}
	if got.statusMessage != "add account failed: oauth failed" {
		t.Fatalf("status = %q, want failure message", got.statusMessage)
	}
	if got.mailbox.Account != original.Account || len(got.mailbox.Labels) != len(original.Labels) {
		t.Fatalf("fake inbox was not preserved: %#v", got.mailbox)
	}
}

func TestReplaceCredentialsCommandShowsLoadingAndSuccess(t *testing.T) {
	calls := 0
	model := New(config.Default(), WithCredentialReplacer(CredentialReplacerFunc(func() error {
		calls++
		return nil
	})))

	updated, cmd := submitTestCommand(model, "replace-credentials")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected replace-credentials command")
	}
	if !got.replacingCreds || got.statusMessage != "replacing credentials..." {
		t.Fatalf("replacing=%v status=%q, want loading state", got.replacingCreds, got.statusMessage)
	}

	updated, followup := got.Update(cmd())
	got = updated.(Model)
	if followup != nil {
		t.Fatalf("expected no followup command, got %T", followup)
	}
	if calls != 1 {
		t.Fatalf("credential replacer calls = %d, want 1", calls)
	}
	if got.replacingCreds || got.statusMessage != "replaced OAuth client credentials" {
		t.Fatalf("replacing=%v status=%q, want success state", got.replacingCreds, got.statusMessage)
	}
}

func TestReplaceCredentialsCommandShowsError(t *testing.T) {
	model := New(config.Default(), WithCredentialReplacer(CredentialReplacerFunc(func() error {
		return errors.New("canceled")
	})))

	updated, cmd := submitTestCommand(model, "replace-credentials")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected replace-credentials command")
	}

	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.replacingCreds {
		t.Fatal("expected loading to finish")
	}
	if got.statusMessage != "replace credentials failed: canceled" {
		t.Fatalf("status = %q, want failure message", got.statusMessage)
	}
}

func TestAddAccountCommandShowsKeyringError(t *testing.T) {
	model := New(config.Default(), WithAccountAdder(AccountAdderFunc(func() (AccountAddResult, error) {
		return AccountAddResult{}, auth.ErrKeyringUnavailable
	})))

	updated, cmd := submitTestCommand(model, "add-account")
	if cmd == nil {
		t.Fatal("expected add-account command")
	}

	updated, _ = updated.(Model).Update(cmd())
	got := updated.(Model)
	want := "add account failed: keychain inaccessible; unlock the OS keychain and retry"
	if got.statusMessage != want || got.toastMessage != want {
		t.Fatalf("status/toast = %q/%q, want %q", got.statusMessage, got.toastMessage, want)
	}
}

func TestOAuthHelpCommand(t *testing.T) {
	model := New(config.Default())

	updated, cmd := submitTestCommand(model, "oauth-help")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected toast timer command")
	}
	if got.statusMessage != oauthHelpMessage || got.toastMessage != oauthHelpMessage {
		t.Fatalf("status/toast = %q/%q, want OAuth help", got.statusMessage, got.toastMessage)
	}
}

func TestCommandModeCapturesUnknownCommand(t *testing.T) {
	model := New(config.Default())

	updated, cmd := submitTestCommand(model, "bogus")
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("expected no command, got %T", cmd)
	}
	if got.statusMessage != "unknown command: bogus" {
		t.Fatalf("status = %q, want unknown command", got.statusMessage)
	}
}

func TestCommandModeRendersInput(t *testing.T) {
	model := New(config.Default())
	updated, _ := model.Update(keyMsg(":"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("a"))
	model = updated.(Model)

	view := model.View()
	if !strings.Contains(view, ":a") {
		t.Fatalf("command input missing from view: %q", view)
	}
}

func submitTestCommand(model Model, command string) (tea.Model, tea.Cmd) {
	updated, _ := model.Update(keyMsg(":"))
	model = updated.(Model)
	for _, r := range command {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	return model.Update(tea.KeyMsg{Type: tea.KeyEnter})
}
