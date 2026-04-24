package ui

import (
	"context"
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

func TestModelNavigationUpdatesMessageSelection(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(keyMsg("j"))
	got := updated.(Model)
	if got.selectedMessage != 1 {
		t.Fatalf("selected message = %d, want 1", got.selectedMessage)
	}

	updated, _ = got.Update(keyMsg("k"))
	got = updated.(Model)
	if got.selectedMessage != 0 {
		t.Fatalf("selected message = %d, want 0", got.selectedMessage)
	}

	updated, _ = got.Update(keyMsg("G"))
	got = updated.(Model)
	if got.selectedMessage != len(got.mailbox.Messages)-1 {
		t.Fatalf("selected message = %d, want last", got.selectedMessage)
	}

	updated, _ = got.Update(keyMsg("g"))
	got = updated.(Model)
	if got.selectedMessage != 0 {
		t.Fatalf("selected message = %d, want 0", got.selectedMessage)
	}
}

func TestModelNavigationUpdatesReaderState(t *testing.T) {
	model := New(config.Default())

	updated, _ := model.Update(keyMsg("enter"))
	got := updated.(Model)

	if !got.readerOpen || got.focus != PaneReader {
		t.Fatalf("readerOpen=%v focus=%v, want reader open and focused", got.readerOpen, got.focus)
	}
}

func TestModelNavigationUpdatesLabelSelection(t *testing.T) {
	model := New(config.Default())
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 16})
	model = updated.(Model)

	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	if model.focus != PaneLabels {
		t.Fatalf("focus=%v, want labels", model.focus)
	}

	updated, _ = model.Update(keyMsg("j"))
	got := updated.(Model)
	if got.selectedLabel != 1 {
		t.Fatalf("selected label = %d, want 1", got.selectedLabel)
	}
}

func TestSearchModeEnterEditSubmitToggleAndExit(t *testing.T) {
	model := New(config.Default())

	updated, cmd := model.Update(keyMsg("/"))
	model = updated.(Model)
	if cmd != nil || model.mode != ModeSearch || model.search.ActiveAccount != model.mailbox.Account {
		t.Fatalf("search enter = mode %v account %q cmd %T", model.mode, model.search.ActiveAccount, cmd)
	}

	for _, r := range "from:alice" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	if model.search.Query != "from:alice" {
		t.Fatalf("query = %q, want from:alice", model.search.Query)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = updated.(Model)
	if model.search.Query != "from:alic" {
		t.Fatalf("query after backspace = %q, want from:alic", model.search.Query)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlUnderscore})
	model = updated.(Model)
	if model.search.Mode != SearchModeOffline || !model.search.Submitted {
		t.Fatalf("toggle state = %s submitted=%v, want offline submitted", model.search.Mode, model.search.Submitted)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !model.search.Submitted || model.statusMessage != "search submitted: from:alic [offline]" {
		t.Fatalf("submit state = submitted %v status %q", model.search.Submitted, model.statusMessage)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.mode != ModeNormal || model.statusMessage != "search canceled" {
		t.Fatalf("exit state = mode %v status %q", model.mode, model.statusMessage)
	}
}

func TestSearchExitRestoresPreviousMailboxContext(t *testing.T) {
	model := New(config.Default())
	model.selectedMessage = 2
	model.focus = PaneReader
	model.readerOpen = true

	updated, _ := model.Update(keyMsg("/"))
	model = updated.(Model)
	model.selectedMessage = 0
	model.focus = PaneList
	model.readerOpen = false

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.selectedMessage != 2 || model.focus != PaneReader || !model.readerOpen {
		t.Fatalf("restored selected=%d focus=%v reader=%v, want prior context", model.selectedMessage, model.focus, model.readerOpen)
	}
}

func TestSearchDefaultsOfflineWhenAccountOffline(t *testing.T) {
	model := New(config.Default())
	model.offline = true

	updated, _ := model.Update(keyMsg("/"))
	got := updated.(Model)

	if got.search.Mode != SearchModeOffline {
		t.Fatalf("search mode = %s, want offline", got.search.Mode)
	}
}

func TestSearchSubmitRunsOnlineSearchAndRendersResults(t *testing.T) {
	runner := &fakeSearchRunner{
		result: SearchResult{
			Messages: []Message{{ID: "msg-1", ThreadID: "thread-1", From: "Ada", Subject: "Search hit", Snippet: "matched text", CacheState: "metadata"}},
		},
	}
	model := New(config.Default(), WithSearchRunner(runner))

	updated, _ := model.Update(keyMsg("/"))
	model = updated.(Model)
	for _, r := range "subject:plan" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil || !model.search.Loading {
		t.Fatalf("search loading=%v cmd=%T, want loading command", model.search.Loading, cmd)
	}
	msg := cmd().(searchDoneMsg)
	updated, _ = model.Update(msg)
	model = updated.(Model)

	if len(runner.requests) != 1 || runner.requests[0].Query != "subject:plan" || runner.requests[0].Mode != SearchModeOnline {
		t.Fatalf("requests = %#v, want online subject query", runner.requests)
	}
	if model.search.Loading || len(model.search.Results) != 1 || model.search.Results[0].Subject != "Search hit" {
		t.Fatalf("results = loading %v %#v, want rendered search hit", model.search.Loading, model.search.Results)
	}
	if model.statusMessage != "search complete: 1 results" {
		t.Fatalf("status = %q, want search complete", model.statusMessage)
	}
}

func TestSearchSubmitCancelsPreviousSearchAndIgnoresStaleResults(t *testing.T) {
	runner := &fakeSearchRunner{}
	model := New(config.Default(), WithSearchRunner(runner))
	updated, _ := model.Update(keyMsg("/"))
	model = updated.(Model)
	for _, r := range "first" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}
	updated, firstCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	firstGeneration := model.searchGeneration

	model.search.Query = "second"
	updated, secondCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if firstCmd == nil || secondCmd == nil {
		t.Fatalf("search commands = %T/%T, want both commands", firstCmd, secondCmd)
	}
	_ = firstCmd()
	if len(runner.canceled) != 1 || !runner.canceled[0] {
		t.Fatalf("cancel state = cmds %T/%T canceled %#v", firstCmd, secondCmd, runner.canceled)
	}

	updated, _ = model.Update(searchDoneMsg{
		Generation: firstGeneration,
		Request:    SearchRequest{Account: model.mailbox.Account, Query: "first", Mode: SearchModeOnline},
		Result:     SearchResult{Messages: []Message{{Subject: "stale"}}},
	})
	model = updated.(Model)
	if len(model.search.Results) != 0 {
		t.Fatalf("results = %#v, want stale result ignored", model.search.Results)
	}
}

func TestSearchToggleRerunsExistingQuery(t *testing.T) {
	runner := &fakeSearchRunner{}
	model := New(config.Default(), WithSearchRunner(runner))
	updated, _ := model.Update(keyMsg("/"))
	model = updated.(Model)
	for _, r := range "from:ada" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}

	updated, cmd := model.Update(keyMsg("ctrl+/"))
	model = updated.(Model)
	if cmd == nil || model.search.Mode != SearchModeOffline || !model.search.Loading {
		t.Fatalf("toggle = mode %s loading %v cmd %T, want offline rerun", model.search.Mode, model.search.Loading, cmd)
	}
	_ = cmd()
	if len(runner.requests) != 1 || runner.requests[0].Mode != SearchModeOffline || runner.requests[0].Query != "from:ada" {
		t.Fatalf("requests = %#v, want offline rerun with query", runner.requests)
	}
}

func TestSearchOfflineDefaultSubmitsOfflineRequest(t *testing.T) {
	runner := &fakeSearchRunner{}
	model := New(config.Default(), WithSearchRunner(runner))
	model.offline = true
	updated, _ := model.Update(keyMsg("/"))
	model = updated.(Model)
	for _, r := range "subject:cached" {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = updated.(Model)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil || model.search.Mode != SearchModeOffline {
		t.Fatalf("submit = mode %s cmd %T, want offline command", model.search.Mode, cmd)
	}
	_ = cmd()
	if len(runner.requests) != 1 || runner.requests[0].Mode != SearchModeOffline {
		t.Fatalf("requests = %#v, want offline request", runner.requests)
	}
}

type fakeSearchRunner struct {
	result   SearchResult
	requests []SearchRequest
	canceled []bool
}

func (f *fakeSearchRunner) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	f.requests = append(f.requests, request)
	if ctx.Err() != nil {
		f.canceled = append(f.canceled, true)
		return SearchResult{}, ctx.Err()
	}
	f.canceled = append(f.canceled, false)
	result := f.result
	result.Account = request.Account
	result.Query = request.Query
	result.Mode = request.Mode
	return result, nil
}

func keyMsg(value string) tea.KeyMsg {
	switch value {
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+/":
		return tea.KeyMsg{Type: tea.KeyCtrlUnderscore}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
