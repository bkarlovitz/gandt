package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	gandtsync "github.com/bkarlovitz/gandt/internal/sync"
	tea "github.com/charmbracelet/bubbletea"
)

func BenchmarkMailboxRender5000(b *testing.B) {
	model := New(config.Default(), WithMailbox(performanceMailbox(5000)))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 40})
	model = updated.(Model)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.View()
	}
}

func BenchmarkColdStartNoAccountView(b *testing.B) {
	cfg := config.Default()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		model := New(cfg, WithMailbox(NoAccountMailbox()))
		updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 40})
		_ = updated.(Model).View()
	}
}

func BenchmarkCachedThreadOpen5000(b *testing.B) {
	model := New(config.Default(), WithMailbox(performanceMailbox(5000)))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 40})
	model = updated.(Model)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updated, cmd := model.Update(keyMsg("enter"))
		if cmd != nil {
			b.Fatal("cached thread open should not load")
		}
		model = updated.(Model)
		model.readerOpen = false
		model.focus = PaneList
	}
}

func BenchmarkSearchResultsRender100(b *testing.B) {
	results := performanceMailbox(100).Messages
	model := New(config.Default())
	model.mode = ModeSearch
	model.search = SearchState{Query: "subject", Mode: SearchModeOffline, Results: results, Submitted: true}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 132, Height: 40})
	model = updated.(Model)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.View()
	}
}

func BenchmarkMailboxMemory10000(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		model := New(config.Default(), WithMailbox(performanceMailbox(10000)))
		if len(model.mailbox.Messages) != 10000 {
			b.Fatal("missing benchmark messages")
		}
	}
}

func BenchmarkTriageOptimisticAction5000(b *testing.B) {
	mailbox := performanceMailbox(5000)
	actor := &fakeTriageActor{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model := New(config.Default(), WithMailbox(cloneMailbox(mailbox)), WithTriageActor(actor))
		_, _ = model.startTriageAction(TriageActionRequest{Kind: TriageStar, Add: true})
	}
}

func TestNavigationDoesNotBlockOnBackgroundSyncDelay(t *testing.T) {
	coordinator := blockingSyncCoordinator{done: make(chan struct{})}
	model := New(config.Default(), WithSyncCoordinator(coordinator))
	syncCmd := model.Init()
	started := make(chan struct{})
	go func() {
		close(started)
		_ = syncCmd()
	}()
	<-started

	start := time.Now()
	updated, _ := model.Update(keyMsg("j"))
	got := updated.(Model)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("navigation update took %s, want below 50ms while sync command blocks", elapsed)
	}
	if got.selectedMessage != 1 {
		t.Fatalf("selected message = %d, want navigation to continue", got.selectedMessage)
	}
	close(coordinator.done)
}

func performanceMailbox(count int) Mailbox {
	messages := make([]Message, 0, count)
	for i := 0; i < count; i++ {
		messages = append(messages, Message{
			ID:          fmt.Sprintf("msg-%04d", i),
			ThreadID:    fmt.Sprintf("thread-%04d", i),
			From:        "Sender",
			Address:     "sender@example.com",
			Subject:     fmt.Sprintf("Subject %04d", i),
			Date:        "Apr 24",
			Snippet:     "Cached message snippet for performance verification.",
			LabelIDs:    []string{"INBOX"},
			Unread:      i%3 == 0,
			ThreadCount: 1,
			CacheState:  "cached",
		})
	}
	return RealAccountMailbox("me@example.com", []Label{{ID: "INBOX", Name: "Inbox", System: true, Unread: count / 3}}, map[string][]Message{
		"INBOX": messages,
	})
}

type blockingSyncCoordinator struct {
	done chan struct{}
}

func (c blockingSyncCoordinator) Next(ctx context.Context, active bool) gandtsync.CoordinatorUpdate {
	select {
	case <-ctx.Done():
		return gandtsync.CoordinatorUpdate{Stopped: true}
	case <-c.done:
		return gandtsync.CoordinatorUpdate{Summary: "sync complete"}
	}
}
