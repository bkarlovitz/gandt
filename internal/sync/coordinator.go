package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
)

type SyncRunner interface {
	RunSync(context.Context) (AccountSyncResult, error)
}

type SyncRunnerFunc func(context.Context) (AccountSyncResult, error)

func (fn SyncRunnerFunc) RunSync(ctx context.Context) (AccountSyncResult, error) {
	return fn(ctx)
}

type Clock interface {
	After(time.Duration) <-chan time.Time
}

type RealClock struct{}

func (RealClock) After(duration time.Duration) <-chan time.Time {
	return time.After(duration)
}

type Coordinator struct {
	runner         SyncRunner
	clock          Clock
	activeInterval time.Duration
	idleInterval   time.Duration
}

type CoordinatorOption func(*Coordinator)

func WithClock(clock Clock) CoordinatorOption {
	return func(c *Coordinator) {
		if clock != nil {
			c.clock = clock
		}
	}
}

type CoordinatorUpdate struct {
	AccountID string
	Summary   string
	Err       error
	Stopped   bool
	Fallback  bool
}

func NewCoordinator(cfg config.Config, runner SyncRunner, opts ...CoordinatorOption) Coordinator {
	defaults := config.Default()
	coordinator := Coordinator{
		runner:         runner,
		clock:          RealClock{},
		activeInterval: time.Duration(firstPositive(cfg.Sync.PollActiveSeconds, defaults.Sync.PollActiveSeconds)) * time.Second,
		idleInterval:   time.Duration(firstPositive(cfg.Sync.PollIdleSeconds, defaults.Sync.PollIdleSeconds)) * time.Second,
	}
	for _, opt := range opts {
		opt(&coordinator)
	}
	return coordinator
}

func (c Coordinator) Next(ctx context.Context, active bool) CoordinatorUpdate {
	interval := c.idleInterval
	if active {
		interval = c.activeInterval
	}
	select {
	case <-ctx.Done():
		return CoordinatorUpdate{Stopped: true}
	case <-c.clock.After(interval):
	}
	if ctx.Err() != nil {
		return CoordinatorUpdate{Stopped: true}
	}
	if c.runner == nil {
		return CoordinatorUpdate{Summary: "sync unavailable"}
	}
	result, err := c.runner.RunSync(ctx)
	if ctx.Err() != nil {
		return CoordinatorUpdate{Stopped: true}
	}
	if err != nil {
		return CoordinatorUpdate{Err: err, Summary: "sync failed: " + err.Error()}
	}
	summary := result.Status
	if summary == "" {
		summary = "sync complete"
	}
	return CoordinatorUpdate{
		Summary:  summary,
		Fallback: result.Fallback,
	}
}

func (c Coordinator) ActiveInterval() time.Duration {
	return c.activeInterval
}

func (c Coordinator) IdleInterval() time.Duration {
	return c.idleInterval
}

func firstPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	panic(fmt.Sprintf("invalid positive default %d", fallback))
}
