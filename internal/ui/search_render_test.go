package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchResultsOnlineSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := searchRenderModel(SearchModeOnline, []Message{{
		ID:              "msg-1",
		ThreadID:        "thread-1",
		From:            "Ada",
		Address:         "ada@example.com",
		Subject:         "Release plan",
		Date:            "Apr 24",
		Snippet:         "Matched cached metadata",
		Unread:          true,
		ThreadCount:     2,
		CacheState:      "metadata",
		AttachmentCount: 1,
	}})

	assertSnapshot(t, model.View(), searchOnlineSnapshot)
}

func TestSearchResultsOfflineSnapshot(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := searchRenderModel(SearchModeOffline, []Message{{
		ID:         "msg-2",
		ThreadID:   "thread-2",
		From:       "Bob",
		Address:    "bob@example.com",
		Subject:    "Offline hit",
		Date:       "Apr 23",
		Snippet:    "Body text matched locally",
		CacheState: "cached",
	}})

	assertSnapshot(t, model.View(), searchOfflineSnapshot)
}

func TestSearchStatesSnapshots(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	loading := searchRenderModel(SearchModeOnline, nil)
	loading.search.Loading = true
	assertSnapshot(t, loading.View(), searchLoadingSnapshot)

	empty := searchRenderModel(SearchModeOffline, nil)
	assertSnapshot(t, empty.View(), searchEmptySnapshot)

	failed := searchRenderModel(SearchModeOffline, nil)
	failed.search.Error = "unsupported offline search operator: has"
	assertSnapshot(t, failed.View(), searchErrorSnapshot)
}

func TestSearchResultsNavigateAndOpenReader(t *testing.T) {
	model := searchRenderModel(SearchModeOnline, []Message{
		{ID: "msg-1", ThreadID: "thread-1", From: "Ada", Subject: "First"},
		{ID: "msg-2", ThreadID: "thread-2", From: "Bob", Subject: "Second", Body: []string{"cached search body"}},
	})

	updated, _ := model.Update(keyMsg("j"))
	model = updated.(Model)
	if model.selectedMessage != 1 {
		t.Fatalf("selected message = %d, want second search result", model.selectedMessage)
	}
	updated, cmd := model.Update(keyMsg("enter"))
	model = updated.(Model)
	if cmd != nil || !model.readerOpen || model.focus != PaneReader {
		t.Fatalf("reader state = open %v focus %v cmd %T", model.readerOpen, model.focus, cmd)
	}
	if !containsLine(model.View(), "cached search body") {
		t.Fatalf("reader view did not render selected search result body:\n%s", model.View())
	}
}

func TestSearchResultsLoadThreadFromSelectedResult(t *testing.T) {
	loader := ThreadLoaderFunc(func(request ThreadLoadRequest) (ThreadLoadResult, error) {
		if request.Message.ID != "msg-2" {
			return ThreadLoadResult{}, errors.New("wrong message loaded")
		}
		return ThreadLoadResult{MessageID: "msg-2", ThreadID: "thread-2", Body: []string{"loaded body"}, CacheState: "cached"}, nil
	})
	model := searchRenderModel(SearchModeOnline, []Message{
		{ID: "msg-1", ThreadID: "thread-1", From: "Ada", Subject: "First", Body: []string{"first"}},
		{ID: "msg-2", ThreadID: "thread-2", From: "Bob", Subject: "Second", CacheState: "metadata"},
	})
	model.threadLoader = loader
	model.selectedMessage = 1

	updated, cmd := model.Update(keyMsg("enter"))
	model = updated.(Model)
	if cmd == nil || model.loadingThreadID != "thread-2" {
		t.Fatalf("load state = %q cmd %T, want thread-2 command", model.loadingThreadID, cmd)
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if len(model.search.Results[1].Body) != 1 || model.search.Results[1].Body[0] != "loaded body" {
		t.Fatalf("search result body = %#v, want loaded body", model.search.Results[1].Body)
	}
}

func searchRenderModel(mode SearchMode, results []Message) Model {
	model := New(config.Default())
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	model = updated.(Model)
	model.mode = ModeSearch
	model.search = SearchState{
		Query:         "subject:plan",
		Mode:          mode,
		ActiveAccount: model.mailbox.Account,
		Results:       results,
		Submitted:     true,
	}
	return model
}

func containsLine(value, needle string) bool {
	return strings.Contains(value, needle)
}

const searchOnlineSnapshot = `G&T | work: me@work.com | search: subject:plan [online]
----------------------------------------------------------------------------------------------------
search: subject:plan [online]          | Reader
> Ada          Apr 24 Release plan ... | From: Ada <ada@example.com>
  metadata | Matched cached metadata   | Subject: Release plan
                                       | Date: Apr 24
                                       |
                                       | [metadata only; body not cached]
----------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const searchOfflineSnapshot = `G&T | work: me@work.com | search: subject:plan [offline]
----------------------------------------------------------------------------------------------------
search: subject:plan [offline]         | Reader
> Bob          Apr 23 Offline hit      | From: Bob <bob@example.com>
  cached | Body text matched locally   | Subject: Offline hit
                                       | Date: Apr 23
                                       |
                                       | [no body]
----------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const searchLoadingSnapshot = `G&T | work: me@work.com | search: subject:plan [online]
----------------------------------------------------------------------------------------------------
search: subject:plan [online]          | Reader
                                       |
Searching...                           | No message selected
----------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const searchEmptySnapshot = `G&T | work: me@work.com | search: subject:plan [offline]
----------------------------------------------------------------------------------------------------
search: subject:plan [offline]         | Reader
                                       |
No search results                      | No message selected
----------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`

const searchErrorSnapshot = `G&T | work: me@work.com | search: subject:plan [offline]
----------------------------------------------------------------------------------------------------
search: subject:plan [offline]         | Reader
                                       |
unsupported offline search operator... | No message selected
----------------------------------------------------------------------------------------------------
j/k: nav   enter: open   tab: pane   ?: help   q: quit`
