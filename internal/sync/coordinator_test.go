package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
)

func TestCoordinatorUsesActiveAndIdleIntervals(t *testing.T) {
	cfg := config.Default()
	cfg.Sync.PollActiveSeconds = 2
	cfg.Sync.PollIdleSeconds = 7
	clock := newFakeClock()
	calls := 0
	coordinator := NewCoordinator(cfg, SyncRunnerFunc(func(context.Context) (AccountSyncResult, error) {
		calls++
		return AccountSyncResult{Status: "synced"}, nil
	}), WithClock(clock))

	activeDone := make(chan CoordinatorUpdate, 1)
	go func() {
		activeDone <- coordinator.Next(context.Background(), true)
	}()
	clock.waitForAfter(t)
	if got := clock.duration(0); got != 2*time.Second {
		t.Fatalf("active interval = %s, want 2s", got)
	}
	clock.tick()
	if update := <-activeDone; update.Summary != "synced" {
		t.Fatalf("active update = %#v, want synced", update)
	}

	idleDone := make(chan CoordinatorUpdate, 1)
	go func() {
		idleDone <- coordinator.Next(context.Background(), false)
	}()
	clock.waitForAfter(t)
	if got := clock.duration(1); got != 7*time.Second {
		t.Fatalf("idle interval = %s, want 7s", got)
	}
	clock.tick()
	if update := <-idleDone; update.Summary != "synced" {
		t.Fatalf("idle update = %#v, want synced", update)
	}
	if calls != 2 {
		t.Fatalf("sync calls = %d, want 2", calls)
	}
}

func TestCoordinatorStopsBeforeNetworkWorkWhenContextCancelled(t *testing.T) {
	cfg := config.Default()
	clock := newFakeClock()
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	coordinator := NewCoordinator(cfg, SyncRunnerFunc(func(context.Context) (AccountSyncResult, error) {
		calls++
		return AccountSyncResult{}, nil
	}), WithClock(clock))

	done := make(chan CoordinatorUpdate, 1)
	go func() {
		done <- coordinator.Next(ctx, true)
	}()
	clock.waitForAfter(t)
	cancel()
	clock.tick()
	update := <-done
	if !update.Stopped {
		t.Fatalf("update = %#v, want stopped", update)
	}
	if calls != 0 {
		t.Fatalf("sync calls = %d, want none after cancellation", calls)
	}
}

func TestCoordinatorSurfacesSyncErrors(t *testing.T) {
	cfg := config.Default()
	clock := newFakeClock()
	coordinator := NewCoordinator(cfg, SyncRunnerFunc(func(context.Context) (AccountSyncResult, error) {
		return AccountSyncResult{}, errors.New("network down")
	}), WithClock(clock))

	done := make(chan CoordinatorUpdate, 1)
	go func() {
		done <- coordinator.Next(context.Background(), true)
	}()
	clock.waitForAfter(t)
	clock.tick()
	update := <-done
	if update.Err == nil || update.Summary != "sync failed: network down" {
		t.Fatalf("update = %#v, want surfaced sync error", update)
	}
}

func TestCoordinatorSurfacesRateLimitExhaustion(t *testing.T) {
	cfg := config.Default()
	clock := newFakeClock()
	coordinator := NewCoordinator(cfg, SyncRunnerFunc(func(context.Context) (AccountSyncResult, error) {
		return AccountSyncResult{}, gmail.ErrRateLimited
	}), WithClock(clock))

	done := make(chan CoordinatorUpdate, 1)
	go func() {
		done <- coordinator.Next(context.Background(), true)
	}()
	clock.waitForAfter(t)
	clock.tick()
	update := <-done
	if !errors.Is(update.Err, gmail.ErrRateLimited) || update.Summary != "sync failed: gmail rate limited" {
		t.Fatalf("update = %#v, want rate-limit sync failure", update)
	}
}

type fakeClock struct {
	afterCalls chan time.Duration
	ticks      chan time.Time
	durations  []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{
		afterCalls: make(chan time.Duration, 10),
		ticks:      make(chan time.Time, 10),
	}
}

func (c *fakeClock) After(duration time.Duration) <-chan time.Time {
	c.durations = append(c.durations, duration)
	c.afterCalls <- duration
	return c.ticks
}

func (c *fakeClock) tick() {
	c.ticks <- time.Now()
}

func (c *fakeClock) waitForAfter(t *testing.T) {
	t.Helper()
	select {
	case <-c.afterCalls:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clock After call")
	}
}

func (c *fakeClock) duration(index int) time.Duration {
	if index >= len(c.durations) {
		return 0
	}
	return c.durations[index]
}
