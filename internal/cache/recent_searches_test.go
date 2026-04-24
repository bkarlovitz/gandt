package cache

import (
	"context"
	"testing"
	"time"
)

func TestRecentSearchRepositoryRecordDedupeOrderingLimitAndScope(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accounts := NewAccountRepository(db)
	first, err := accounts.Create(ctx, CreateAccountParams{Email: "first@example.com"})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	second, err := accounts.Create(ctx, CreateAccountParams{Email: "second@example.com"})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}
	repo := NewRecentSearchRepository(db)
	base := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	fixtures := []RecentSearch{
		{AccountID: first.ID, Query: "from:ada", Mode: "online", LastUsed: base},
		{AccountID: first.ID, Query: "subject:plan", Mode: "offline", LastUsed: base.Add(time.Minute)},
		{AccountID: first.ID, Query: "to:team", Mode: "online", LastUsed: base.Add(2 * time.Minute)},
		{AccountID: second.ID, Query: "from:ada", Mode: "online", LastUsed: base.Add(3 * time.Minute)},
		{AccountID: first.ID, Query: "from:ada", Mode: "online", LastUsed: base.Add(4 * time.Minute)},
	}
	for _, fixture := range fixtures {
		if err := repo.Record(ctx, fixture, 2); err != nil {
			t.Fatalf("record recent search: %v", err)
		}
	}

	firstSearches, err := repo.List(ctx, first.ID, 10)
	if err != nil {
		t.Fatalf("list first searches: %v", err)
	}
	if len(firstSearches) != 2 {
		t.Fatalf("first searches = %#v, want two after trim", firstSearches)
	}
	if firstSearches[0].Query != "from:ada" || firstSearches[0].Mode != "online" {
		t.Fatalf("first search = %#v, want deduped newest from:ada online", firstSearches[0])
	}
	if firstSearches[1].Query != "to:team" {
		t.Fatalf("second search = %#v, want to:team retained by recency", firstSearches[1])
	}

	secondSearches, err := repo.List(ctx, second.ID, 10)
	if err != nil {
		t.Fatalf("list second searches: %v", err)
	}
	if len(secondSearches) != 1 || secondSearches[0].AccountID != second.ID {
		t.Fatalf("second searches = %#v, want account scoped row", secondSearches)
	}
}

func TestRecentSearchRepositoryDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	account, err := NewAccountRepository(db).Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	repo := NewRecentSearchRepository(db)
	if err := repo.Record(ctx, RecentSearch{AccountID: account.ID, Query: "from:ada", Mode: "offline"}, 10); err != nil {
		t.Fatalf("record recent search: %v", err)
	}
	if err := repo.Delete(ctx, account.ID, "from:ada", "offline"); err != nil {
		t.Fatalf("delete recent search: %v", err)
	}
	searches, err := repo.List(ctx, account.ID, 10)
	if err != nil {
		t.Fatalf("list recent searches: %v", err)
	}
	if len(searches) != 0 {
		t.Fatalf("searches = %#v, want deleted", searches)
	}
}
