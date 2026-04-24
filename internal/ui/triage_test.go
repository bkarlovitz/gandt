package ui

import (
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestTriageActionsApplyOptimisticStateAndDispatchAsync(t *testing.T) {
	tests := []struct {
		name    string
		request TriageActionRequest
		start   Message
		assert  func(*testing.T, Model)
	}{
		{
			name:    "archive",
			request: TriageActionRequest{Kind: TriageArchive},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if len(got.mailbox.Messages) != 0 {
					t.Fatalf("messages = %#v, want archived message removed", got.mailbox.Messages)
				}
			},
		},
		{
			name:    "trash",
			request: TriageActionRequest{Kind: TriageTrash},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if len(got.mailbox.Messages) != 0 {
					t.Fatalf("messages = %#v, want trashed message removed", got.mailbox.Messages)
				}
			},
		},
		{
			name:    "spam",
			request: TriageActionRequest{Kind: TriageSpam},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if len(got.mailbox.Messages) != 0 {
					t.Fatalf("messages = %#v, want spammed message removed", got.mailbox.Messages)
				}
			},
		},
		{
			name:    "star",
			request: TriageActionRequest{Kind: TriageStar, Add: true},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if !got.mailbox.Messages[0].Starred || !hasLabel(got.mailbox.Messages[0].LabelIDs, "STARRED") {
					t.Fatalf("message = %#v, want starred", got.mailbox.Messages[0])
				}
			},
		},
		{
			name:    "unread",
			request: TriageActionRequest{Kind: TriageUnread, Add: false},
			start:   unreadActionMessage(),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if got.mailbox.Messages[0].Unread || hasLabel(got.mailbox.Messages[0].LabelIDs, "UNREAD") {
					t.Fatalf("message = %#v, want marked read", got.mailbox.Messages[0])
				}
			},
		},
		{
			name:    "label add",
			request: TriageActionRequest{Kind: TriageLabelAdd, LabelID: "Label_1"},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if !hasLabel(got.mailbox.Messages[0].LabelIDs, "Label_1") {
					t.Fatalf("labels = %#v, want Label_1 added", got.mailbox.Messages[0].LabelIDs)
				}
			},
		},
		{
			name:    "label remove",
			request: TriageActionRequest{Kind: TriageLabelRemove, LabelID: "Label_1"},
			start:   actionMessage([]string{"INBOX", "Label_1"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if hasLabel(got.mailbox.Messages[0].LabelIDs, "Label_1") {
					t.Fatalf("labels = %#v, want Label_1 removed", got.mailbox.Messages[0].LabelIDs)
				}
			},
		},
		{
			name:    "mute",
			request: TriageActionRequest{Kind: TriageMute},
			start:   actionMessage([]string{"INBOX"}),
			assert: func(t *testing.T, got Model) {
				t.Helper()
				if !got.mailbox.Messages[0].Muted || !hasLabel(got.mailbox.Messages[0].LabelIDs, "MUTED") {
					t.Fatalf("message = %#v, want muted", got.mailbox.Messages[0])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := &fakeTriageActor{result: TriageActionResult{Summary: "done"}}
			model := actionModel(tt.start, actor)
			updated, cmd := model.startTriageAction(tt.request)
			got := updated.(Model)
			if cmd == nil {
				t.Fatal("expected action command")
			}
			tt.assert(t, got)
			if len(actor.requests) != 0 {
				t.Fatalf("actor ran before command execution: %#v", actor.requests)
			}
			updated, _ = got.Update(cmd())
			got = updated.(Model)
			if got.statusMessage != "done" {
				t.Fatalf("status = %q, want done", got.statusMessage)
			}
			if len(actor.requests) != 1 || actor.requests[0].Kind != tt.request.Kind {
				t.Fatalf("requests = %#v, want dispatched action", actor.requests)
			}
		})
	}
}

func TestTriageActionRevertsLocalStateOnFailure(t *testing.T) {
	actor := &fakeTriageActor{err: errors.New("api down")}
	model := actionModel(actionMessage([]string{"INBOX"}), actor)

	updated, cmd := model.startTriageAction(TriageActionRequest{Kind: TriageStar, Add: true})
	got := updated.(Model)
	if !got.mailbox.Messages[0].Starred {
		t.Fatal("expected optimistic star before API result")
	}
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.mailbox.Messages[0].Starred || got.statusMessage != "action failed: api down" {
		t.Fatalf("state/status = %#v/%q, want reverted failure", got.mailbox.Messages[0], got.statusMessage)
	}
}

func actionModel(message Message, actor TriageActor) Model {
	mailbox := RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true}}, map[string][]Message{
		"INBOX": {message},
	})
	return New(config.Default(), WithMailbox(mailbox), WithTriageActor(actor))
}

func actionMessage(labels []string) Message {
	return Message{ID: "msg-1", ThreadID: "thread-1", Subject: "Subject", LabelIDs: append([]string{}, labels...)}
}

func unreadActionMessage() Message {
	message := actionMessage([]string{"INBOX", "UNREAD"})
	message.Unread = true
	return message
}

func hasLabel(labels []string, labelID string) bool {
	for _, label := range labels {
		if label == labelID {
			return true
		}
	}
	return false
}

type fakeTriageActor struct {
	requests []TriageActionRequest
	result   TriageActionResult
	err      error
}

func (f *fakeTriageActor) ApplyAction(request TriageActionRequest) (TriageActionResult, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return TriageActionResult{}, f.err
	}
	return f.result, nil
}
