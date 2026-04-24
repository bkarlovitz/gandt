package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInitHasNoCommand(t *testing.T) {
	model := New(config.Default())
	if cmd := model.Init(); cmd != nil {
		t.Fatalf("expected nil init command, got %T", cmd)
	}
}

func TestModelUpdateQuit(t *testing.T) {
	model := New(config.Default())

	updated, cmd := model.Update(keyMsg("q"))
	got := updated.(Model)

	if !got.quitting {
		t.Fatal("expected model to be quitting")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestModelUpdateResize(t *testing.T) {
	model := New(config.Default())

	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("expected no command, got %T", cmd)
	}
	if got.width != 120 || got.height != 40 {
		t.Fatalf("got size %dx%d, want 120x40", got.width, got.height)
	}
}

func keyMsg(value string) tea.KeyMsg {
	if value == "ctrl+c" {
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
