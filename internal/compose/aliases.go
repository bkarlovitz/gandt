package compose

import (
	"context"
	"sync"
)

type AliasFetcher func(context.Context, string) ([]Address, error)

type AliasResolver struct {
	mu    sync.Mutex
	fetch AliasFetcher
	cache map[string][]Address
}

func NewAliasResolver(fetch AliasFetcher) *AliasResolver {
	return &AliasResolver{
		fetch: fetch,
		cache: map[string][]Address{},
	}
}

func (r *AliasResolver) Aliases(ctx context.Context, accountID string, accountEmail string) ([]Address, error) {
	fallback := []Address{NewAddress(accountEmail)}
	if r == nil {
		return fallback, nil
	}

	r.mu.Lock()
	if cached, ok := r.cache[accountID]; ok {
		r.mu.Unlock()
		return cloneAddresses(cached), nil
	}
	r.mu.Unlock()

	if r.fetch == nil {
		return fallback, nil
	}
	aliases, err := r.fetch(ctx, accountID)
	if err != nil || len(aliases) == 0 {
		return fallback, err
	}

	r.mu.Lock()
	r.cache[accountID] = cloneAddresses(aliases)
	r.mu.Unlock()
	return cloneAddresses(aliases), nil
}

func cloneAddresses(addresses []Address) []Address {
	out := make([]Address, len(addresses))
	copy(out, addresses)
	return out
}
