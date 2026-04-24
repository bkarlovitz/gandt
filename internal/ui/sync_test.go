package ui

import (
	"context"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	gandtsync "github.com/bkarlovitz/gandt/internal/sync"
)

func TestModelInitStartsBackgroundSync(t *testing.T) {
	coordinator := &fakeSyncCoordinator{
		updates: []gandtsync.CoordinatorUpdate{
			{Summary: "sync complete"},
			{Summary: "idle sync complete"},
		},
	}
	model := New(config.Default(), WithSyncCoordinator(coordinator))

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected sync init command")
	}
	msg, ok := cmd().(SyncUpdateMsg)
	if !ok {
		t.Fatalf("init msg = %T, want SyncUpdateMsg", cmd())
	}
	updated, followup := model.Update(msg)
	model = updated.(Model)
	if model.statusMessage != "sync complete" {
		t.Fatalf("status = %q, want sync complete", model.statusMessage)
	}
	if followup == nil {
		t.Fatal("expected follow-up sync command")
	}
	_ = followup()
	if len(coordinator.activeCalls) != 2 || !coordinator.activeCalls[0] || coordinator.activeCalls[1] {
		t.Fatalf("active calls = %#v, want init active then idle", coordinator.activeCalls)
	}
}

func TestModelMarksSyncActiveAfterInput(t *testing.T) {
	coordinator := &fakeSyncCoordinator{
		updates: []gandtsync.CoordinatorUpdate{
			{Summary: "sync complete"},
			{Summary: "active sync complete"},
		},
	}
	model := New(config.Default(), WithSyncCoordinator(coordinator))
	msg := model.Init()().(SyncUpdateMsg)

	updated, _ := model.Update(keyMsg("j"))
	model = updated.(Model)
	updated, followup := model.Update(msg)
	model = updated.(Model)
	if followup == nil {
		t.Fatal("expected follow-up sync command")
	}
	_ = followup()
	if len(coordinator.activeCalls) != 2 || !coordinator.activeCalls[1] {
		t.Fatalf("active calls = %#v, want second cycle active after key input", coordinator.activeCalls)
	}
}

func TestModelStopsSyncLoopOnStoppedMessage(t *testing.T) {
	model := New(config.Default(), WithSyncCoordinator(&fakeSyncCoordinator{
		updates: []gandtsync.CoordinatorUpdate{{Stopped: true}},
	}))
	msg := model.Init()().(SyncUpdateMsg)

	updated, followup := model.Update(msg)
	got := updated.(Model)
	if followup != nil {
		t.Fatalf("expected no follow-up command after stop, got %T", followup)
	}
	if got.statusMessage != "" {
		t.Fatalf("status = %q, want unchanged", got.statusMessage)
	}
}

type fakeSyncCoordinator struct {
	updates     []gandtsync.CoordinatorUpdate
	activeCalls []bool
}

func (f *fakeSyncCoordinator) Next(ctx context.Context, active bool) gandtsync.CoordinatorUpdate {
	f.activeCalls = append(f.activeCalls, active)
	if len(f.updates) == 0 {
		return gandtsync.CoordinatorUpdate{Summary: "sync complete"}
	}
	update := f.updates[0]
	f.updates = f.updates[1:]
	return update
}
