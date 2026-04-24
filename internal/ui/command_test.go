package ui

import (
	"errors"
	"strings"
	"testing"

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
	if !got.addingAccount || got.statusMessage != "adding account..." {
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
