package compose

import (
	"context"
	"errors"
	"testing"
)

func TestAliasResolverMemoizesByAccount(t *testing.T) {
	calls := map[string]int{}
	resolver := NewAliasResolver(func(_ context.Context, accountID string) ([]Address, error) {
		calls[accountID]++
		return []Address{NewAddress(accountID + "@example.com")}, nil
	})

	first, err := resolver.Aliases(context.Background(), "acct-a", "me@example.com")
	if err != nil {
		t.Fatalf("first aliases: %v", err)
	}
	second, err := resolver.Aliases(context.Background(), "acct-a", "me@example.com")
	if err != nil {
		t.Fatalf("second aliases: %v", err)
	}
	other, err := resolver.Aliases(context.Background(), "acct-b", "other@example.com")
	if err != nil {
		t.Fatalf("other aliases: %v", err)
	}

	if calls["acct-a"] != 1 {
		t.Fatalf("acct-a calls = %d, want memoized once", calls["acct-a"])
	}
	if calls["acct-b"] != 1 {
		t.Fatalf("acct-b calls = %d, want separate account fetch", calls["acct-b"])
	}
	if first[0].Email != "acct-a@example.com" || second[0].Email != "acct-a@example.com" || other[0].Email != "acct-b@example.com" {
		t.Fatalf("aliases not scoped by account: first=%v second=%v other=%v", first, second, other)
	}
}

func TestAliasResolverFallsBackToAccountEmail(t *testing.T) {
	fetchErr := errors.New("gmail unavailable")
	resolver := NewAliasResolver(func(_ context.Context, _ string) ([]Address, error) {
		return nil, fetchErr
	})

	aliases, err := resolver.Aliases(context.Background(), "acct-a", "me@example.com")
	if !errors.Is(err, fetchErr) {
		t.Fatalf("error = %v, want fetch error for caller visibility", err)
	}
	if len(aliases) != 1 || aliases[0].Email != "me@example.com" {
		t.Fatalf("fallback aliases = %#v, want account email", aliases)
	}
}
