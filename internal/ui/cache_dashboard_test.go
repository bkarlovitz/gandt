package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCacheCommandLoadsDashboard(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	calls := 0
	model := New(config.Default(), WithCacheDashboardLoader(CacheDashboardLoaderFunc(func() (CacheDashboard, error) {
		calls++
		return cacheDashboardFixture(4), nil
	})))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)

	updated, cmd := submitTestCommand(model, "cache")
	got := updated.(Model)
	if cmd == nil {
		t.Fatal("expected cache dashboard command")
	}
	if !got.loadingCacheDashboard || got.statusMessage != "loading cache dashboard..." {
		t.Fatalf("loading=%v status=%q, want loading dashboard", got.loadingCacheDashboard, got.statusMessage)
	}

	updated, followup := got.Update(cmd())
	got = updated.(Model)
	if followup != nil {
		t.Fatalf("expected no followup command, got %T", followup)
	}
	if calls != 1 {
		t.Fatalf("loader calls = %d, want 1", calls)
	}
	if got.mode != ModeCacheDashboard || got.loadingCacheDashboard {
		t.Fatalf("mode/loading = %v/%v, want cache dashboard and not loading", got.mode, got.loadingCacheDashboard)
	}
	view := got.View()
	for _, want := range []string{"cache dashboard", "Messages 20", "Accounts", "Labels", "F  Inbox", "Age", "Rows"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q:\n%s", want, view)
		}
	}
}

func TestCacheCommandShowsLoaderError(t *testing.T) {
	model := New(config.Default(), WithCacheDashboardLoader(CacheDashboardLoaderFunc(func() (CacheDashboard, error) {
		return CacheDashboard{}, errors.New("cache unavailable")
	})))

	updated, cmd := submitTestCommand(model, "cache")
	if cmd == nil {
		t.Fatal("expected cache dashboard command")
	}
	updated, _ = updated.(Model).Update(cmd())
	got := updated.(Model)
	if got.mode != ModeNormal || got.loadingCacheDashboard {
		t.Fatalf("mode/loading = %v/%v, want normal and not loading", got.mode, got.loadingCacheDashboard)
	}
	if got.statusMessage != "cache dashboard failed: cache unavailable" {
		t.Fatalf("status = %q, want loader error", got.statusMessage)
	}
}

func BenchmarkCacheDashboardRenderLarge(b *testing.B) {
	model := New(config.Default())
	model.mode = ModeCacheDashboard
	model.cacheDashboard = cacheDashboardFixture(5000)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.View()
	}
}

func TestCacheDashboardRenderLargeUnder200ms(t *testing.T) {
	model := New(config.Default())
	model.mode = ModeCacheDashboard
	model.cacheDashboard = cacheDashboardFixture(5000)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	start := time.Now()
	_ = model.View()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("cache dashboard render took %s, want below 200ms", elapsed)
	}
}

func cacheDashboardFixture(labels int) CacheDashboard {
	dashboard := CacheDashboard{
		GeneratedAt:           time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		SQLiteBytes:           128 * 1024,
		TotalBytes:            64 * 1024,
		MessageCount:          labels * 5,
		BodyCount:             labels * 4,
		AttachmentCount:       labels,
		CachedAttachmentCount: labels / 2,
		FTSBytes:              4096,
		FTSRows:               labels * 5,
		Accounts: []CacheDashboardAccount{
			{Email: "me@example.com", MessageCount: labels * 5, BodyCount: labels * 4, AttachmentCount: labels, TotalBytes: 64 * 1024},
		},
		Ages: []CacheDashboardAge{
			{Bucket: "0-7d", MessageCount: labels, BodyCount: labels, AttachmentCount: labels / 5, TotalBytes: 16 * 1024},
			{Bucket: "8-30d", MessageCount: labels * 2, BodyCount: labels, AttachmentCount: labels / 4, TotalBytes: 24 * 1024},
		},
		Rows: []CacheDashboardRow{
			{Table: "messages", Rows: labels * 5},
			{Table: "message_labels", Rows: labels * 5},
			{Table: "attachments", Rows: labels},
			{Table: "messages_fts", Rows: labels * 5},
		},
	}
	for i := 0; i < labels; i++ {
		depth := "metadata"
		if i == 0 {
			depth = "full"
		} else if i%2 == 0 {
			depth = "body"
		}
		name := fmt.Sprintf("Label %04d", i)
		id := fmt.Sprintf("Label_%04d", i)
		if i == 0 {
			name = "Inbox"
			id = "INBOX"
		}
		dashboard.Labels = append(dashboard.Labels, CacheDashboardLabel{
			AccountEmail:    "me@example.com",
			LabelID:         id,
			LabelName:       name,
			CacheDepth:      depth,
			MessageCount:    5,
			BodyCount:       4,
			AttachmentCount: 1,
			AttachmentBytes: 1024,
			TotalBytes:      2048,
		})
	}
	return dashboard
}
