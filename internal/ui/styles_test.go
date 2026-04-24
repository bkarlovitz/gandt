package ui

import (
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestDarkThemeUsesSemanticRoleColors(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	cfg := config.Default()
	cfg.UI.Theme = config.ThemeDark
	model := themedStateModel(cfg)

	view := resizedView(model, 96, 12)
	for _, want := range []string{
		"38;2;215;248;243", // header primary
		"48;2;36;92;89",    // selected accent
		"38;2;255;255;255", // unread foreground
		"38;2;142;163;161", // read/muted foreground
		"38;2;255;107;107", // error status
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("dark theme view missing %s:\n%q", want, view)
		}
	}
}

func TestLightThemeUsesSemanticRoleColors(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	cfg := config.Default()
	cfg.UI.Theme = config.ThemeLight
	model := themedStateModel(cfg)

	view := resizedView(model, 96, 12)
	for _, want := range []string{
		"38;2;31;40;55",    // header primary
		"48;2;217;242;239", // selected accent
		"38;2;17;24;39",    // unread foreground
		"38;2;102;112;133", // read/muted foreground
		"38;2;179;35;24",   // error status
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("light theme view missing %s:\n%q", want, view)
		}
	}
}

func TestNoColorDisablesANSIAcrossViews(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := themedStateModel(config.Default())
	model.cacheDashboard = CacheDashboard{MessageCount: 1}
	model.cachePolicyTable = CachePolicyTable{Rows: []CachePolicyRow{{AccountEmail: "me@example.com", LabelName: "Inbox", Depth: "full"}}}

	tests := []struct {
		name string
		mode Mode
	}{
		{name: "mailbox", mode: ModeNormal},
		{name: "help", mode: ModeHelp},
		{name: "command", mode: ModeCommand},
		{name: "account switcher", mode: ModeAccountSwitcher},
		{name: "cache dashboard", mode: ModeCacheDashboard},
		{name: "cache policy", mode: ModeCachePolicyEditor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.mode = tt.mode
			view := resizedView(model, 100, 14)
			if strings.Contains(view, "\x1b[") {
				t.Fatalf("NO_COLOR view contains ANSI:\n%q", view)
			}
		})
	}
}

func themedStateModel(cfg config.Config) Model {
	message := readerMessage("message-1", "thread-1", []string{"body"})
	message.Unread = true
	read := readerMessage("message-2", "thread-2", []string{"read body"})
	read.From = "Bob"
	read.Unread = false
	muted := readerMessage("message-3", "thread-3", []string{"muted body"})
	muted.From = "Muted"
	muted.Muted = true
	model := New(cfg, WithMailbox(readerMailboxWithMessages([]Message{message, read, muted})))
	model.selectedMessage = 1
	model.statusMessage = "sync failed: quota"
	return model
}

func resizedView(model Model, width int, height int) string {
	updated, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model).View()
}
