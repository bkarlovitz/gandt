package ui

import (
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestAccountColorTintsActiveFrame(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	model := New(config.Default(), WithAccounts([]AccountState{{
		Account: "me@example.com",
		Color:   "#4285f4",
		Mailbox: RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}}),
	}}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	view := model.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("view has no ANSI styling: %q", view)
	}
	if !strings.Contains(view, "38;2;65;133;243") {
		t.Fatalf("view missing active account foreground color: %q", view)
	}
}

func TestConfiguredAccountColorCanOverrideDefaultAccent(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	model := New(config.Default(), WithAccounts([]AccountState{{
		Account: "me@example.com",
		Color:   "#123456",
		Mailbox: RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}}),
	}}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	view := model.View()
	if !strings.Contains(view, "38;2;18;52;86") {
		t.Fatalf("view missing configured account color: %q", view)
	}
}

func TestAccountColorPreservesNoColorOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := New(config.Default(), WithAccounts([]AccountState{{
		Account: "me@example.com",
		Color:   "#4285f4",
		Mailbox: RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}}),
	}}))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	view := model.View()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("NO_COLOR view contains ANSI styling: %q", view)
	}
}
