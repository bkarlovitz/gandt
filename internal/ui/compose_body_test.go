package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestComposeBodyEditorInlineEditing(t *testing.T) {
	editor := NewComposeBodyEditor("hello", 80, 12, nil)
	updated, _ := editor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})

	if got := updated.Body(); got != "hello!" {
		t.Fatalf("body = %q, want typed text preserved in textarea", got)
	}
	if updated.DraftSaved() {
		t.Fatal("typing should dirty a saved draft state")
	}
}

func TestComposeBodyEditorExternalHandoff(t *testing.T) {
	editor := NewComposeBodyEditor("draft", 80, 12, func(body string) (string, error) {
		return body + "\nedited", nil
	})

	updated, cmd := editor.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if cmd == nil {
		t.Fatal("expected external editor command")
	}
	msg := cmd()
	updated, _ = updated.Update(msg)

	if got := updated.Body(); got != "draft\nedited" {
		t.Fatalf("body = %q, want external edit result", got)
	}
	if updated.ValidationError() != "" {
		t.Fatalf("validation error = %q, want none", updated.ValidationError())
	}
}

func TestComposeBodyEditorExternalHandoffErrorKeepsBody(t *testing.T) {
	editor := NewComposeBodyEditor("draft", 80, 12, func(body string) (string, error) {
		return body + "\nuntrusted", errors.New("editor failed")
	})

	updated, cmd := editor.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	updated, _ = updated.Update(cmd())

	if got := updated.Body(); got != "draft" {
		t.Fatalf("body = %q, want original body on external error", got)
	}
	if !strings.Contains(updated.ValidationError(), "editor failed") {
		t.Fatalf("validation error = %q, want editor failure", updated.ValidationError())
	}
}

func TestComposeBodyEditorPreservesBodyAcrossResizeValidationAndDraftSave(t *testing.T) {
	editor := NewComposeBodyEditor("draft body", 10, 1, nil)
	updated, _ := editor.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := updated.Body(); got != "draft body" {
		t.Fatalf("body after resize = %q", got)
	}
	if width, height := updated.Size(); width != 100 || height != 30 {
		t.Fatalf("size = %dx%d, want 100x30", width, height)
	}

	updated = updated.WithValidationError(errors.New("body is required"))
	if got := updated.Body(); got != "draft body" {
		t.Fatalf("body after validation = %q", got)
	}

	updated, body := updated.SaveDraft()
	if body != "draft body" || !updated.DraftSaved() || updated.ValidationError() != "" {
		t.Fatalf("draft save body=%q saved=%v err=%q", body, updated.DraftSaved(), updated.ValidationError())
	}
}

func TestComposeBodyEditorCancelConfirmation(t *testing.T) {
	editor := NewComposeBodyEditor("draft", 80, 12, nil)
	updated, _ := editor.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !updated.CancelConfirming() {
		t.Fatal("expected cancel confirmation state")
	}
	if got := updated.Body(); got != "draft" {
		t.Fatalf("body after cancel request = %q", got)
	}
}
