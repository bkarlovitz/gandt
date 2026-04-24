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

func readerMailbox(body []string) Mailbox {
	return RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true, CacheDepth: "metadata"}}, map[string][]Message{
		"INBOX": {
			{
				ID:         "message-1",
				ThreadID:   "thread-1",
				From:       "Ada",
				Address:    "ada@example.com",
				Subject:    "Needs body",
				Date:       "Apr 24",
				Snippet:    "metadata row",
				Body:       body,
				CacheState: "metadata",
			},
		},
	})
}
