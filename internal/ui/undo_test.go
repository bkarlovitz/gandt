package ui

import (
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestUndoRestoresPreviousStateAndDispatchesInverse(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	actor := &fakeTriageActor{result: TriageActionResult{Summary: "done"}}
	model := actionModel(actionMessage([]string{"INBOX"}), actor)
	model = New(config.Default(),
		WithMailbox(model.mailbox),
		WithTriageActor(actor),
		WithNow(func() time.Time { return now }),
	)

	updated, cmd := model.startTriageAction(TriageActionRequest{Kind: TriageStar, Add: true})
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if !model.mailbox.Messages[0].Starred {
		t.Fatal("expected starred message before undo")
	}

	updated, cmd = model.Update(keyMsg("U"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected undo command")
	}
	if model.mailbox.Messages[0].Starred {
		t.Fatal("expected undo to restore unstarred state immediately")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.statusMessage != "undone" {
		t.Fatalf("status = %q, want undone", model.statusMessage)
	}
	if len(actor.requests) != 2 || actor.requests[1].Kind != TriageStar || actor.requests[1].Add || !actor.requests[1].Undo {
		t.Fatalf("requests = %#v, want inverse star undo", actor.requests)
	}
}

func TestUndoRestoresArchivedMessageAndAddsInboxBack(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	actor := &fakeTriageActor{}
	model := actionModel(actionMessage([]string{"INBOX"}), actor)
	model = New(config.Default(),
		WithMailbox(model.mailbox),
		WithTriageActor(actor),
		WithNow(func() time.Time { return now }),
	)

	updated, cmd := model.startTriageAction(TriageActionRequest{Kind: TriageArchive})
	model = updated.(Model)
	if len(model.mailbox.Messages) != 0 {
		t.Fatalf("messages = %#v, want archived message removed", model.mailbox.Messages)
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	updated, cmd = model.Update(keyMsg("U"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected undo command")
	}
	if len(model.mailbox.Messages) != 1 {
		t.Fatalf("messages = %#v, want archived message restored", model.mailbox.Messages)
	}
	_ = cmd()
	if len(actor.requests) != 2 || actor.requests[1].Kind != TriageLabelAdd || actor.requests[1].LabelID != "INBOX" {
		t.Fatalf("requests = %#v, want INBOX add undo", actor.requests)
	}
}

func TestUndoExpiresAfterWindow(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	actor := &fakeTriageActor{}
	model := actionModel(actionMessage([]string{"INBOX"}), actor)
	model = New(config.Default(),
		WithMailbox(model.mailbox),
		WithTriageActor(actor),
		WithNow(func() time.Time { return now }),
	)

	updated, cmd := model.startTriageAction(TriageActionRequest{Kind: TriageStar, Add: true})
	model = updated.(Model)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	now = now.Add(31 * time.Second)

	updated, cmd = model.Update(keyMsg("U"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected toast dismissal command for expired undo")
	}
	if model.statusMessage != "undo expired" {
		t.Fatalf("status = %q, want undo expired", model.statusMessage)
	}
}

func TestUndoUnavailableMessaging(t *testing.T) {
	model := actionModel(actionMessage([]string{"INBOX"}), &fakeTriageActor{})

	updated, cmd := model.Update(keyMsg("U"))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected toast dismissal command for unavailable undo")
	}
	if got.statusMessage != "nothing to undo" {
		t.Fatalf("status = %q, want nothing to undo", got.statusMessage)
	}
}
