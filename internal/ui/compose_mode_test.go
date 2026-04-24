package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/compose"
	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestComposeModeKeyPaths(t *testing.T) {
	model := New(config.Default())

	for _, tc := range []struct {
		key  string
		kind compose.ComposeKind
	}{
		{"c", compose.ComposeKindNew},
		{"r", compose.ComposeKindReply},
		{"R", compose.ComposeKindReplyAll},
		{"f", compose.ComposeKindForward},
	} {
		updated, cmd := model.Update(keyMsg(tc.key))
		got := updated.(Model)
		if cmd != nil {
			t.Fatalf("%s command = %T, want nil", tc.key, cmd)
		}
		if got.mode != ModeCompose || got.compose.Kind != tc.kind {
			t.Fatalf("%s compose mode=%v kind=%s, want %s", tc.key, got.mode, got.compose.Kind, tc.kind)
		}
	}
}

func TestComposeModeSaveSendDiscardAndAttach(t *testing.T) {
	model := New(config.Default())
	updated, _ := model.Update(keyMsg("c"))
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	model = updated.(Model)
	if !model.compose.AttachPrompt || model.statusMessage != "attach file" {
		t.Fatalf("attach state=%v status=%q", model.compose.AttachPrompt, model.statusMessage)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	got := updated.(Model)
	if got.mode != ModeNormal || got.compose.SendStatus != compose.SendStatusSent || got.statusMessage != "send complete" {
		t.Fatalf("send state mode=%v status=%s msg=%q", got.mode, got.compose.SendStatus, got.statusMessage)
	}

	updated, _ = model.Update(keyMsg("c"))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got = updated.(Model)
	if got.mode != ModeNormal || got.compose.SendStatus != compose.SendStatusDraftSaved || got.statusMessage != "draft saved" {
		t.Fatalf("draft state mode=%v status=%s msg=%q", got.mode, got.compose.SendStatus, got.statusMessage)
	}

	updated, _ = model.Update(keyMsg("c"))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model = updated.(Model)
	if !model.compose.DiscardConfirm || !strings.Contains(model.View(), "Discard? y/n") {
		t.Fatalf("discard confirm state=%v view=%q", model.compose.DiscardConfirm, model.View())
	}
	updated, _ = model.Update(keyMsg("y"))
	got = updated.(Model)
	if got.mode != ModeNormal || got.statusMessage != "compose discarded" {
		t.Fatalf("discard state mode=%v msg=%q", got.mode, got.statusMessage)
	}
}

func TestComposeModeAttachmentPathInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	model := New(config.Default())
	updated, _ := model.Update(keyMsg("c"))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	model = updated.(Model)
	for _, r := range path {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if len(model.compose.Attachments) != 1 || model.compose.Attachments[0].Filename != "note.txt" {
		t.Fatalf("attachments = %#v", model.compose.Attachments)
	}
	if !strings.Contains(model.View(), "note.txt") || !strings.Contains(model.View(), "bytes") {
		t.Fatalf("view = %q, want attachment metadata", model.View())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	model = updated.(Model)
	for _, r := range filepath.Join(filepath.Dir(path), "missing.txt") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.compose.Error == "" || !strings.Contains(model.statusMessage, "attach failed") {
		t.Fatalf("error=%q status=%q", model.compose.Error, model.statusMessage)
	}
}
