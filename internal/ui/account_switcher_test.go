package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestAccountSwitcherRendersCachedAccountSummaries(t *testing.T) {
	model := switcherTestModel()
	model.width = 100

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	got := updated.(Model)
	if cmd != nil || got.mode != ModeAccountSwitcher {
		t.Fatalf("cmd/mode = %T/%v, want switcher mode", cmd, got.mode)
	}

	view := got.View()
	for _, want := range []string{"account switcher", "1  [#4285f4] Work", "work@example.com", "2  [#0f9d58] personal@example.com", "3 unread"} {
		if !strings.Contains(view, want) {
			t.Fatalf("switcher view missing %q:\n%s", want, view)
		}
	}
}

func TestAccountSwitcherSwitchesFromCachedStateWithoutCommand(t *testing.T) {
	model := switcherTestModel()
	model.mode = ModeAccountSwitcher

	start := time.Now()
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	elapsed := time.Since(start)
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("switch returned command %T, want cached model update only", cmd)
	}
	if elapsed >= 50*time.Millisecond {
		t.Fatalf("switch took %s, want under 50ms", elapsed)
	}
	if got.mailbox.Account != "personal@example.com" || got.mode != ModeNormal {
		t.Fatalf("mailbox/mode = %q/%v, want personal normal", got.mailbox.Account, got.mode)
	}
	if got.statusMessage != "switched to personal@example.com" {
		t.Fatalf("status = %q, want switched status", got.statusMessage)
	}
}

func TestAccountDigitShortcutSwitchesWithoutOpeningOverlay(t *testing.T) {
	model := switcherTestModel()

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	got := updated.(Model)
	if cmd != nil || got.mode != ModeNormal || got.mailbox.Account != "personal@example.com" {
		t.Fatalf("cmd/mode/account = %T/%v/%q, want direct cached switch", cmd, got.mode, got.mailbox.Account)
	}
}

func switcherTestModel() Model {
	return New(config.Default(), WithAccounts([]AccountState{
		{
			Account:     "work@example.com",
			DisplayName: "Work",
			Color:       "#4285f4",
			SyncStatus:  "synced",
			Mailbox: RealAccountMailbox("work@example.com", []Label{
				{ID: "INBOX", Name: "Inbox", Unread: 3, System: true},
			}),
		},
		{
			Account: "personal@example.com",
			Color:   "#0f9d58",
			Mailbox: RealAccountMailbox("personal@example.com", []Label{
				{ID: "INBOX", Name: "Inbox", Unread: 1, System: true},
			}),
		},
	}))
}
