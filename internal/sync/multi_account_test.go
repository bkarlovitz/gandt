package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/bkarlovitz/gandt/internal/cache"
)

func TestRunAccountsIndependentlyContinuesAfterAccountFailure(t *testing.T) {
	ctx := context.Background()
	accounts := []cache.Account{
		{ID: "first", Email: "first@example.com"},
		{ID: "second", Email: "second@example.com"},
		{ID: "third", Email: "third@example.com"},
	}
	calls := []string{}

	result, err := RunAccountsIndependently(ctx, accounts, AccountRunnerFunc(func(ctx context.Context, account cache.Account) (AccountSyncResult, error) {
		calls = append(calls, account.ID)
		if account.ID == "second" {
			return AccountSyncResult{}, errors.New("second failed")
		}
		return AccountSyncResult{AccountID: account.ID, Status: "synced"}, nil
	}))
	if err != nil {
		t.Fatalf("run accounts: %v", err)
	}
	if result.Status != "sync-all complete: 2 synced, 1 failed" {
		t.Fatalf("status = %q, want partial failure summary", result.Status)
	}
	if len(calls) != 3 || calls[0] != "first" || calls[1] != "second" || calls[2] != "third" {
		t.Fatalf("calls = %#v, want all accounts attempted", calls)
	}
}

func TestRunAccountsIndependentlyReturnsErrorWhenAllAccountsFail(t *testing.T) {
	ctx := context.Background()
	accounts := []cache.Account{{ID: "first"}, {ID: "second"}}

	_, err := RunAccountsIndependently(ctx, accounts, AccountRunnerFunc(func(context.Context, cache.Account) (AccountSyncResult, error) {
		return AccountSyncResult{}, errors.New("network down")
	}))
	if err == nil || err.Error() != "network down" {
		t.Fatalf("err = %v, want first account error", err)
	}
}
