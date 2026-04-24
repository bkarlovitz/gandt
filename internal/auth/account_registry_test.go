package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bkarlovitz/gandt/internal/cache"
)

func TestWriteAccountRegistryPersistsNonSecretAccountData(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	accounts := cache.NewAccountRepository(db)
	first, err := accounts.Create(ctx, cache.CreateAccountParams{Email: "one@example.com", DisplayName: "One", Color: "#4285f4"})
	if err != nil {
		t.Fatalf("create first account: %v", err)
	}
	second, err := accounts.Create(ctx, cache.CreateAccountParams{Email: "two@example.com", Color: "#0f9d58"})
	if err != nil {
		t.Fatalf("create second account: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config", "accounts.json")
	if err := WriteAccountRegistry(ctx, accounts, path); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var registry AccountRegistryFile
	if err := json.Unmarshal(body, &registry); err != nil {
		t.Fatalf("decode registry: %v", err)
	}
	if len(registry.Accounts) != 2 {
		t.Fatalf("registry accounts = %#v, want 2", registry.Accounts)
	}
	if registry.Accounts[0].ID != first.ID || registry.Accounts[0].Email != "one@example.com" || registry.Accounts[0].DisplayName != "One" {
		t.Fatalf("first registry entry = %#v", registry.Accounts[0])
	}
	if registry.Accounts[1].ID != second.ID || registry.Accounts[1].Email != "two@example.com" {
		t.Fatalf("second registry entry = %#v", registry.Accounts[1])
	}
	if string(body) == "" || json.Valid(body) == false {
		t.Fatalf("registry is not valid JSON: %s", body)
	}
}
