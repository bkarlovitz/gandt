package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkarlovitz/gandt/internal/cache"
)

type AccountRegistryEntry struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Color       string `json:"color,omitempty"`
}

type AccountRegistryFile struct {
	Accounts []AccountRegistryEntry `json:"accounts"`
}

func WriteAccountRegistry(ctx context.Context, accounts cache.AccountRepository, path string) error {
	if path == "" {
		return nil
	}
	listed, err := accounts.List(ctx)
	if err != nil {
		return err
	}

	registry := AccountRegistryFile{Accounts: make([]AccountRegistryEntry, 0, len(listed))}
	for _, account := range listed {
		registry.Accounts = append(registry.Accounts, AccountRegistryEntry{
			ID:          account.ID,
			Email:       account.Email,
			DisplayName: account.DisplayName,
			Color:       account.Color,
		})
	}

	body, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode account registry: %w", err)
	}
	body = append(body, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create account registry directory: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write account registry: %w", err)
	}
	return nil
}
