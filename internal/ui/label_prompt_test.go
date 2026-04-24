package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestLabelAddPromptCreatesLabelAndAppliesSelection(t *testing.T) {
	actor := &fakeTriageActor{result: TriageActionResult{Summary: "label added", LabelID: "Label_real", LabelName: "Projects"}}
	message := actionMessage([]string{"INBOX"})
	message.Unread = true
	model := actionModel(message, actor)
	model = New(config.Default(), WithMailbox(model.mailbox), WithTriageActor(actor))

	updated, cmd := model.Update(keyMsg("+"))
	model = updated.(Model)
	if cmd != nil || model.mode != ModeLabelPrompt || model.statusMessage != "add label" {
		t.Fatalf("prompt state = mode %v status %q cmd %T, want add label prompt", model.mode, model.statusMessage, cmd)
	}
	for _, r := range "Projects" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected label add command")
	}
	if !hasLabel(model.mailbox.Messages[0].LabelIDs, "Label_Projects") {
		t.Fatalf("labels = %#v, want generated label added", model.mailbox.Messages[0].LabelIDs)
	}
	if len(model.mailbox.Labels) != 2 || model.mailbox.Labels[1].Name != "Projects" || model.mailbox.Labels[1].Unread != 1 {
		t.Fatalf("labels = %#v, want Projects label count updated", model.mailbox.Labels)
	}
	_ = cmd()
	if len(actor.requests) != 1 || !actor.requests[0].CreateLabel || actor.requests[0].LabelName != "Projects" {
		t.Fatalf("requests = %#v, want create label action", actor.requests)
	}
}

func TestLabelCreateReconcilesGmailLabelIDAfterSuccess(t *testing.T) {
	actor := &fakeTriageActor{result: TriageActionResult{Summary: "label added", LabelID: "Label_123", LabelName: "Projects"}}
	model := actionModel(actionMessage([]string{"INBOX"}), actor)
	model = New(config.Default(), WithMailbox(model.mailbox), WithTriageActor(actor))

	updated, cmd := model.startTriageAction(TriageActionRequest{
		Kind:        TriageLabelAdd,
		LabelID:     "Label_Projects",
		LabelName:   "Projects",
		Add:         true,
		CreateLabel: true,
	})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected label add command")
	}
	if !hasLabel(model.mailbox.Messages[0].LabelIDs, "Label_Projects") {
		t.Fatalf("labels = %#v, want temporary optimistic label", model.mailbox.Messages[0].LabelIDs)
	}

	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if hasLabel(model.mailbox.Messages[0].LabelIDs, "Label_Projects") || !hasLabel(model.mailbox.Messages[0].LabelIDs, "Label_123") {
		t.Fatalf("labels = %#v, want real Gmail label ID after success", model.mailbox.Messages[0].LabelIDs)
	}
	if labelKey(model.mailbox.Labels[1]) != "Label_123" {
		t.Fatalf("labels = %#v, want sidebar label reconciled to Gmail ID", model.mailbox.Labels)
	}
	if _, ok := model.mailbox.MessagesByLabel["Label_123"]; !ok {
		t.Fatalf("messages by label = %#v, want real Gmail label key", model.mailbox.MessagesByLabel)
	}
}

func TestLabelRemovePromptRemovesSelectedLabel(t *testing.T) {
	actor := &fakeTriageActor{result: TriageActionResult{Summary: "label removed"}}
	message := actionMessage([]string{"INBOX", "Label_1"})
	message.Unread = true
	mailbox := RealAccountMailbox("me@example.com", []Label{
		{ID: "INBOX", Name: "Inbox", System: true},
		{ID: "Label_1", Name: "Projects", Unread: 1},
	}, map[string][]Message{
		"INBOX":   {message},
		"Label_1": {message},
	})
	model := New(config.Default(), WithMailbox(mailbox), WithTriageActor(actor))

	updated, cmd := model.Update(keyMsg("-"))
	model = updated.(Model)
	if cmd != nil || model.mode != ModeLabelPrompt {
		t.Fatalf("prompt state = mode %v cmd %T, want remove label prompt", model.mode, cmd)
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected label remove command")
	}
	if hasLabel(model.mailbox.Messages[0].LabelIDs, "Label_1") || len(model.mailbox.MessagesByLabel["Label_1"]) != 0 || model.mailbox.Labels[1].Unread != 0 {
		t.Fatalf("mailbox = %#v, want Label_1 removed and counts updated", model.mailbox)
	}
	_ = cmd()
	if len(actor.requests) != 1 || actor.requests[0].Kind != TriageLabelRemove || actor.requests[0].LabelID != "Label_1" {
		t.Fatalf("requests = %#v, want label remove action", actor.requests)
	}
}

func TestLabelPromptCancelReturnsToNormalMode(t *testing.T) {
	model := actionModel(actionMessage([]string{"INBOX"}), &fakeTriageActor{})

	updated, _ := model.Update(keyMsg("+"))
	model = updated.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if cmd != nil || model.mode != ModeNormal || model.statusMessage != "label canceled" {
		t.Fatalf("cancel state = mode %v status %q cmd %T, want normal canceled", model.mode, model.statusMessage, cmd)
	}
}

func TestLabelRemoveCurrentLabelUpdatesSelection(t *testing.T) {
	actor := &fakeTriageActor{}
	message := actionMessage([]string{"INBOX", "Label_1"})
	mailbox := RealAccountMailbox("me@example.com", []Label{
		{ID: "INBOX", Name: "Inbox", System: true},
		{ID: "Label_1", Name: "Projects"},
	}, map[string][]Message{
		"INBOX":   {message},
		"Label_1": {message},
	})
	model := New(config.Default(), WithMailbox(mailbox), WithTriageActor(actor))
	model.selectedLabel = 1
	model.updateSelectedLabelMessages()

	updated, cmd := model.startTriageAction(TriageActionRequest{Kind: TriageLabelRemove, LabelID: "Label_1"})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected remove command")
	}
	if len(model.mailbox.Messages) != 0 || model.selectedMessage != 0 {
		t.Fatalf("messages=%#v selected=%d, want removed current-label selection", model.mailbox.Messages, model.selectedMessage)
	}
}
