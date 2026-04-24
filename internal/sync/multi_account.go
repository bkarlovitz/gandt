package sync

import (
	"context"
	"fmt"

	"github.com/bkarlovitz/gandt/internal/cache"
)

type AccountRunner interface {
	RunAccountSync(context.Context, cache.Account) (AccountSyncResult, error)
}

type AccountRunnerFunc func(context.Context, cache.Account) (AccountSyncResult, error)

func (fn AccountRunnerFunc) RunAccountSync(ctx context.Context, account cache.Account) (AccountSyncResult, error) {
	return fn(ctx, account)
}

func RunAccountsIndependently(ctx context.Context, accounts []cache.Account, runner AccountRunner) (AccountSyncResult, error) {
	if len(accounts) == 0 {
		return AccountSyncResult{Status: "sync skipped: no accounts configured"}, nil
	}
	if runner == nil {
		return AccountSyncResult{}, fmt.Errorf("account sync runner is required")
	}

	synced := 0
	failed := 0
	var firstErr error
	for _, account := range accounts {
		result, err := runner.RunAccountSync(ctx, account)
		if ctx.Err() != nil {
			return AccountSyncResult{}, ctx.Err()
		}
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if result.Fallback {
			failed++
		} else {
			synced++
		}
	}
	if synced == 0 && failed > 0 && firstErr != nil {
		return AccountSyncResult{}, firstErr
	}
	status := fmt.Sprintf("sync-all complete: %d synced", synced)
	if failed > 0 {
		status += fmt.Sprintf(", %d failed", failed)
	}
	return AccountSyncResult{Status: status}, nil
}
