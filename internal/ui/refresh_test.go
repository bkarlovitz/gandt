package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestRefreshKeyStartsDeltaSyncAndShowsSuccess(t *testing.T) {
	refresher := &fakeManualRefresher{result: RefreshResult{Summary: "delta synced"}}
	model := New(config.Default(), WithManualRefresher(refresher))

	updated, cmd := model.Update(keyMsg("ctrl+r"))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if got.statusMessage != "syncing..." || got.toastMessage != "syncing..." || got.refreshingAccount == "" {
		t.Fatalf("refresh state = status %q toast %q account %q, want progress", got.statusMessage, got.toastMessage, got.refreshingAccount)
	}

	updated, followup := got.Update(cmd())
	got = updated.(Model)
	if followup != nil {
		t.Fatalf("unexpected follow-up command %T", followup)
	}
	if got.refreshingAccount != "" || got.statusMessage != "delta synced" || got.toastMessage != "delta synced" {
		t.Fatalf("done state = status %q toast %q account %q, want success", got.statusMessage, got.toastMessage, got.refreshingAccount)
	}
	if len(refresher.requests) != 1 || refresher.requests[0].Kind != RefreshDelta {
		t.Fatalf("requests = %#v, want delta refresh", refresher.requests)
	}
}

func TestRelistKeyRefreshesCurrentLabel(t *testing.T) {
	refresher := &fakeManualRefresher{result: RefreshResult{Summary: "label refreshed"}}
	model := New(config.Default(), WithManualRefresher(refresher))

	updated, cmd := submitTestCommand(model, "sync-label")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected relist command")
	}
	if got.statusMessage != "refreshing Inbox..." {
		t.Fatalf("status = %q, want label progress", got.statusMessage)
	}
	_ = cmd()
	if len(refresher.requests) != 1 || refresher.requests[0].Kind != RefreshRelistLabel || refresher.requests[0].LabelName != "Inbox" {
		t.Fatalf("requests = %#v, want relist current Inbox", refresher.requests)
	}
}

func TestSyncAllCommandSubmitsManualRefresh(t *testing.T) {
	refresher := &fakeManualRefresher{result: RefreshResult{Summary: "all synced"}}
	model := New(config.Default(), WithManualRefresher(refresher))

	updated, cmd := submitTestCommand(model, "sync-all")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected sync-all command")
	}
	if got.statusMessage != "syncing all accounts..." {
		t.Fatalf("status = %q, want sync-all progress", got.statusMessage)
	}
	_ = cmd()
	if len(refresher.requests) != 1 || refresher.requests[0].Kind != RefreshAll {
		t.Fatalf("requests = %#v, want sync-all refresh", refresher.requests)
	}
}

func TestRefreshPreventsOverlappingSyncForSameAccount(t *testing.T) {
	model := New(config.Default(), WithManualRefresher(&fakeManualRefresher{}))

	updated, cmd := model.Update(keyMsg("ctrl+r"))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected first refresh command")
	}
	updated, second := got.Update(keyMsg("ctrl+r"))
	got = updated.(Model)
	if second != nil {
		t.Fatalf("expected overlapping refresh to be debounced, got %T", second)
	}
	if !strings.Contains(got.statusMessage, "sync already running") {
		t.Fatalf("status = %q, want overlap warning", got.statusMessage)
	}
}

func TestRefreshErrorRendersToast(t *testing.T) {
	refresher := &fakeManualRefresher{err: errors.New("quota")}
	model := New(config.Default(), WithManualRefresher(refresher))

	updated, cmd := model.Update(keyMsg("ctrl+r"))
	got := updated.(Model)
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	view := got.View()
	if got.statusMessage != "sync failed: quota" || !strings.Contains(view, "sync failed: quota") {
		t.Fatalf("status/view = %q/%q, want rendered refresh error toast", got.statusMessage, view)
	}
}

type fakeManualRefresher struct {
	requests []RefreshRequest
	result   RefreshResult
	err      error
}

func (f *fakeManualRefresher) Refresh(request RefreshRequest) (RefreshResult, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return RefreshResult{}, f.err
	}
	return f.result, nil
}
