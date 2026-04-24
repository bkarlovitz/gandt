package ui

import (
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCachePolicyCommandEditsSavesAndRefreshes(t *testing.T) {
	store := &recordingCachePolicyStore{table: CachePolicyTable{Rows: []CachePolicyRow{
		{AccountID: "acct", AccountEmail: "me@example.com", LabelID: "INBOX", LabelName: "Inbox", Depth: "metadata", AttachmentRule: "none"},
	}}}
	refreshCalls := 0
	model := New(config.Default(),
		WithCachePolicyStore(store),
		WithManualRefresher(ManualRefresherFunc(func(request RefreshRequest) (RefreshResult, error) {
			refreshCalls++
			if request.Kind != RefreshAll {
				t.Fatalf("refresh kind = %s, want all", request.Kind)
			}
			return RefreshResult{Summary: "sync refreshed"}, nil
		})),
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	model = updated.(Model)

	updated, cmd := submitTestCommand(model, "cache-policy")
	if cmd == nil {
		t.Fatal("expected cache policy load command")
	}
	updated, _ = updated.(Model).Update(cmd())
	model = updated.(Model)
	if model.mode != ModeCachePolicyEditor {
		t.Fatalf("mode = %v, want policy editor", model.mode)
	}

	updated, _ = model.Update(keyMsg("d"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("t"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("a"))
	model = updated.(Model)
	row := model.cachePolicyTable.Rows[0]
	if row.Depth != "body" || row.RetentionDays == nil || *row.RetentionDays != 30 || row.AttachmentRule != "under_size" || row.AttachmentMaxMB == nil || *row.AttachmentMaxMB != 10 {
		t.Fatalf("edited row = %#v, want body/30/under_size 10MB", row)
	}

	updated, saveCmd := model.Update(keyMsg("s"))
	model = updated.(Model)
	if saveCmd == nil || !model.savingCachePolicy {
		t.Fatalf("save cmd/loading = %T/%v, want save command and saving state", saveCmd, model.savingCachePolicy)
	}
	updated, refreshCmd := model.Update(saveCmd())
	model = updated.(Model)
	if refreshCmd == nil {
		t.Fatal("expected refresh command after save")
	}
	updated, _ = model.Update(refreshCmd())
	model = updated.(Model)

	if len(store.saved) != 1 || store.saved[0].Depth != "body" || store.saved[0].RetentionDays == nil || *store.saved[0].RetentionDays != 30 {
		t.Fatalf("saved rows = %#v, want edited row persisted", store.saved)
	}
	if !model.cachePolicyTable.Rows[0].Explicit {
		t.Fatalf("saved row = %#v, want explicit row", model.cachePolicyTable.Rows[0])
	}
	if refreshCalls != 1 || model.statusMessage != "sync refreshed" {
		t.Fatalf("refresh calls/status = %d/%q, want one refresh completion", refreshCalls, model.statusMessage)
	}
}

func TestCachePolicyEditorCancelDoesNotSave(t *testing.T) {
	store := &recordingCachePolicyStore{table: CachePolicyTable{Rows: []CachePolicyRow{
		{AccountID: "acct", AccountEmail: "me@example.com", LabelID: "INBOX", LabelName: "Inbox", Depth: "metadata", AttachmentRule: "none"},
	}}}
	model := New(config.Default(), WithCachePolicyStore(store))

	updated, cmd := submitTestCommand(model, "cache-policy")
	updated, _ = updated.(Model).Update(cmd())
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("d"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("esc"))
	model = updated.(Model)

	if model.mode != ModeNormal || len(store.saved) != 0 {
		t.Fatalf("mode/saved = %v/%#v, want normal mode and no persisted rows", model.mode, store.saved)
	}
}

func TestCachePolicyEditorResetUsesDefault(t *testing.T) {
	store := &recordingCachePolicyStore{table: CachePolicyTable{Rows: []CachePolicyRow{
		{AccountID: "acct", AccountEmail: "me@example.com", LabelID: "INBOX", LabelName: "Inbox", Explicit: true, Depth: "full", AttachmentRule: "all"},
	}}}
	model := New(config.Default(), WithCachePolicyStore(store))

	updated, cmd := submitTestCommand(model, "cache-policy")
	updated, _ = updated.(Model).Update(cmd())
	model = updated.(Model)
	updated, resetCmd := model.Update(keyMsg("x"))
	model = updated.(Model)
	if resetCmd == nil {
		t.Fatal("expected reset command")
	}
	updated, _ = model.Update(resetCmd())
	model = updated.(Model)

	if len(store.reset) != 1 {
		t.Fatalf("reset rows = %#v, want one reset", store.reset)
	}
	row := model.cachePolicyTable.Rows[0]
	if row.Explicit || row.Depth != "metadata" || row.AttachmentRule != "none" {
		t.Fatalf("reset row = %#v, want inherited metadata policy", row)
	}
}

type recordingCachePolicyStore struct {
	table CachePolicyTable
	saved []CachePolicyRow
	reset []CachePolicyRow
}

func (s *recordingCachePolicyStore) LoadCachePolicies() (CachePolicyTable, error) {
	return s.table, nil
}

func (s *recordingCachePolicyStore) SaveCachePolicy(row CachePolicyRow) (CachePolicyRow, error) {
	s.saved = append(s.saved, row)
	row.Explicit = true
	s.table.Rows[0] = row
	return row, nil
}

func (s *recordingCachePolicyStore) ResetCachePolicy(row CachePolicyRow) (CachePolicyRow, error) {
	s.reset = append(s.reset, row)
	row.Explicit = false
	row.Depth = "metadata"
	row.RetentionDays = intValue(365)
	row.AttachmentRule = "none"
	row.AttachmentMaxMB = nil
	s.table.Rows[0] = row
	return row, nil
}
