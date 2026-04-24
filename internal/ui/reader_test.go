package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestReaderOpensCachedBodyWithoutLoading(t *testing.T) {
	calls := 0
	model := New(config.Default(),
		WithMailbox(readerMailbox([]string{"cached body"})),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			calls++
			return ThreadLoadResult{}, nil
		})),
	)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	if cmd != nil {
		t.Fatalf("cmd = %T, want nil for cached body", cmd)
	}
	if calls != 0 {
		t.Fatalf("loader calls = %d, want 0", calls)
	}
	if !got.readerOpen || got.focus != PaneReader {
		t.Fatalf("readerOpen=%v focus=%v, want reader", got.readerOpen, got.focus)
	}
}

func TestReaderStartsLoadingOnCacheMiss(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailbox(nil)),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{MessageID: "message-1", ThreadID: "thread-1", Body: []string{"loaded body"}, CacheState: "cached"}, nil
		})),
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	model = updated.(Model)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected thread load command")
	}
	if got.loadingThreadID != "thread-1" || got.statusMessage != "loading thread..." {
		t.Fatalf("loading state = %q/%q, want thread-1 loading", got.loadingThreadID, got.statusMessage)
	}
	if !strings.Contains(got.View(), "Loading thread...") {
		t.Fatalf("view does not show loading state:\n%s", got.View())
	}
}

func TestReaderAppliesLoadedThread(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailbox(nil)),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{
				MessageID:  "message-1",
				ThreadID:   "thread-1",
				Body:       []string{"loaded body"},
				CacheState: "cached",
				Attachments: []Attachment{
					{Name: "plan.pdf", Size: "1.5 KB"},
				},
			}, nil
		})),
	)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.loadingThreadID != "" || got.statusMessage != "loaded thread" {
		t.Fatalf("load state = %q/%q, want loaded", got.loadingThreadID, got.statusMessage)
	}
	message := got.mailbox.Messages[0]
	if len(message.Body) != 1 || message.Body[0] != "loaded body" || message.CacheState != "cached" {
		t.Fatalf("message = %#v, want loaded cached body", message)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("attachments = %#v, want one attachment", message.Attachments)
	}
}

func TestReaderSelectsLoadedThreadMessage(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailboxWithMessages([]Message{
			readerMessage("message-2", "thread-1", nil),
		})),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{
				MessageID:  "message-2",
				ThreadID:   "thread-1",
				CacheState: "cached",
				ThreadMessages: []ThreadMessage{
					{ID: "message-1", From: "Ada", Body: []string{"older"}},
					{ID: "message-2", From: "Bob", Body: []string{"selected"}},
				},
			}, nil
		})),
	)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	updated, _ = got.Update(cmd())
	got = updated.(Model)

	if got.selectedThreadMessage != 1 {
		t.Fatalf("selected thread message = %d, want loaded message index", got.selectedThreadMessage)
	}
	if !strings.Contains(got.View(), "selected") {
		t.Fatalf("view did not open selected loaded message:\n%s", got.View())
	}
}

func TestReaderLoadError(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailbox(nil)),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{}, errors.New("network down")
		})),
	)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	msg := cmd()
	updated, _ = got.Update(msg)
	got = updated.(Model)

	if got.loadingThreadID != "" {
		t.Fatalf("loading thread = %q, want cleared", got.loadingThreadID)
	}
	if got.statusMessage != "load thread failed: network down" {
		t.Fatalf("status = %q, want load failure", got.statusMessage)
	}
}

func TestReaderOfflineErrorKeepsCachedBrowsing(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailboxWithMessages([]Message{
			readerMessage("message-1", "thread-1", nil),
			readerMessage("message-2", "thread-2", []string{"cached second body"}),
		})),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{}, MarkOffline(errors.New("dial tcp failed"))
		})),
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	model = updated.(Model)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if !got.offline || got.statusMessage != "offline: cached mail available" {
		t.Fatalf("offline state = %v/%q, want offline cached status", got.offline, got.statusMessage)
	}
	if !strings.Contains(got.View(), "offline") {
		t.Fatalf("view does not expose offline state:\n%s", got.View())
	}

	updated, _ = got.Update(keyMsg("j"))
	got = updated.(Model)
	if got.selectedMessage != 1 {
		t.Fatalf("selected message = %d, want cached browsing to continue", got.selectedMessage)
	}
}

func TestReaderMarksStreamedButNotCachedBody(t *testing.T) {
	model := New(config.Default(),
		WithMailbox(readerMailbox(nil)),
		WithThreadLoader(ThreadLoaderFunc(func(ThreadLoadRequest) (ThreadLoadResult, error) {
			return ThreadLoadResult{MessageID: "message-1", ThreadID: "thread-1", Body: []string{"streamed body"}, CacheState: "streamed"}, nil
		})),
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	model = updated.(Model)

	updated, cmd := model.Update(keyMsg("enter"))
	got := updated.(Model)
	updated, _ = got.Update(cmd())
	got = updated.(Model)
	if got.mailbox.Messages[0].CacheState != "streamed" {
		t.Fatalf("cache state = %q, want streamed", got.mailbox.Messages[0].CacheState)
	}
	if !strings.Contains(got.View(), "streamed | metadata row") {
		t.Fatalf("view does not mark streamed row:\n%s", got.View())
	}
}

func TestReadOnlyThreadNavigationKeys(t *testing.T) {
	message := readerMessage("message-1", "thread-1", nil)
	message.ThreadMessages = []ThreadMessage{
		{From: "Ada", Address: "ada@example.com", Date: "Apr 24", Body: []string{"first"}},
		{From: "Bob", Address: "bob@example.com", Date: "Apr 24", Body: []string{"second"}},
	}
	model := New(config.Default(), WithMailbox(readerMailboxWithMessages([]Message{message})))

	updated, _ := model.Update(keyMsg("J"))
	model = updated.(Model)
	if model.selectedThreadMessage != 1 {
		t.Fatalf("selected thread message = %d, want 1", model.selectedThreadMessage)
	}
	updated, _ = model.Update(keyMsg("K"))
	model = updated.(Model)
	if model.selectedThreadMessage != 0 {
		t.Fatalf("selected thread message = %d, want 0", model.selectedThreadMessage)
	}
}

func TestReadOnlyNextPreviousThreadKeys(t *testing.T) {
	model := New(config.Default(), WithMailbox(readerMailboxWithMessages([]Message{
		readerMessage("message-1", "thread-1", []string{"first"}),
		readerMessage("message-2", "thread-2", []string{"second"}),
	})))

	updated, _ := model.Update(keyMsg("N"))
	model = updated.(Model)
	if model.selectedMessage != 1 || model.focus != PaneReader {
		t.Fatalf("selected=%d focus=%v, want next reader", model.selectedMessage, model.focus)
	}
	updated, _ = model.Update(keyMsg("P"))
	model = updated.(Model)
	if model.selectedMessage != 0 || model.focus != PaneReader {
		t.Fatalf("selected=%d focus=%v, want previous reader", model.selectedMessage, model.focus)
	}
}

func TestReadOnlyRenderBrowserAndQuoteKeys(t *testing.T) {
	body := []string{
		"reply",
		"> q1",
		"> q2",
		"> q3",
		"> q4",
		"> q5",
	}
	model := New(config.Default(), WithMailbox(readerMailbox(body)))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	model = updated.(Model)

	if !strings.Contains(model.View(), "[quoted text collapsed; 1 lines hidden]") {
		t.Fatalf("view did not collapse long quote:\n%s", model.View())
	}
	updated, _ = model.Update(keyMsg("z"))
	model = updated.(Model)
	if !model.showQuotes || model.statusMessage != "quotes shown" {
		t.Fatalf("quote state = %v/%q, want shown", model.showQuotes, model.statusMessage)
	}
	if !strings.Contains(model.View(), "> q5") {
		t.Fatalf("view did not show expanded quote:\n%s", model.View())
	}

	updated, _ = model.Update(keyMsg("V"))
	model = updated.(Model)
	if model.renderMode != "html2text" || model.statusMessage != "render mode: html2text" {
		t.Fatalf("render mode = %q/%q, want html2text", model.renderMode, model.statusMessage)
	}
	updated, _ = model.Update(keyMsg("B"))
	model = updated.(Model)
	if model.statusMessage != "browser open unavailable in read-only mode" {
		t.Fatalf("browser status = %q, want unavailable placeholder", model.statusMessage)
	}
}

func TestNarrowReaderListFocusToggle(t *testing.T) {
	model := New(config.Default(), WithMailbox(readerMailbox([]string{"body"})))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 72, Height: 12})
	model = updated.(Model)

	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	if model.focus != PaneReader || !model.readerOpen {
		t.Fatalf("focus=%v readerOpen=%v, want reader below 80 cols", model.focus, model.readerOpen)
	}
	updated, _ = model.Update(keyMsg("tab"))
	model = updated.(Model)
	if model.focus != PaneList || model.readerOpen {
		t.Fatalf("focus=%v readerOpen=%v, want list below 80 cols", model.focus, model.readerOpen)
	}
}

func readerMailbox(body []string) Mailbox {
	return readerMailboxWithMessages([]Message{readerMessage("message-1", "thread-1", body)})
}

func readerMailboxWithMessages(messages []Message) Mailbox {
	return RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true, CacheDepth: "metadata"}}, map[string][]Message{
		"INBOX": messages,
	})
}

func readerMessage(id string, threadID string, body []string) Message {
	return Message{
		ID:         id,
		ThreadID:   threadID,
		From:       "Ada",
		Address:    "ada@example.com",
		Subject:    "Needs body",
		Date:       "Apr 24",
		Snippet:    "metadata row",
		Body:       body,
		CacheState: "metadata",
	}
}
