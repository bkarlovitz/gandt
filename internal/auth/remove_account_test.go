package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

func TestAccountRemoverRevokesDeletesTokenRowsAndAttachments(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	store := NewSecretStore(newFakeKeyring())
	account := seedRemovableAccount(t, ctx, db, store, "me@example.com")
	attachmentRoot := t.TempDir()
	attachmentPath := filepath.Join(attachmentRoot, account.ID, "message-1", "file.txt")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o700); err != nil {
		t.Fatalf("create attachment dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("cached"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	registryPath := filepath.Join(t.TempDir(), "accounts.json")
	revoked := false
	remover := NewAccountRemover(db, store, registryPath, attachmentRoot, TokenRevokerFunc(func(ctx context.Context, token *oauth2.Token) error {
		revoked = token.RefreshToken == "refresh-me@example.com"
		return nil
	}))

	result, err := remover.Remove(ctx, "me@example.com")
	if err != nil {
		t.Fatalf("remove account: %v", err)
	}
	if result.Account.ID != account.ID || result.RevokeFailed {
		t.Fatalf("result = %#v, want removed account without revoke failure", result)
	}
	if !revoked {
		t.Fatal("token was not revoked")
	}
	if _, err := store.OAuthToken(account.ID); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("token after remove = %v, want ErrSecretNotFound", err)
	}
	if _, err := cache.NewAccountRepository(db).Get(ctx, account.ID); !errors.Is(err, cache.ErrAccountNotFound) {
		t.Fatalf("account after remove = %v, want ErrAccountNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(attachmentRoot, account.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("attachment dir stat = %v, want removed", err)
	}
	body, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	if string(body) != "{\n  \"accounts\": []\n}\n" {
		t.Fatalf("registry = %s, want empty accounts", body)
	}
}

func TestAccountRemoverContinuesAfterRevokeFailure(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	store := NewSecretStore(newFakeKeyring())
	account := seedRemovableAccount(t, ctx, db, store, "me@example.com")
	remover := NewAccountRemover(db, store, "", "", TokenRevokerFunc(func(context.Context, *oauth2.Token) error {
		return errors.New("network down")
	}))

	result, err := remover.Remove(ctx, account.ID)
	if err != nil {
		t.Fatalf("remove account: %v", err)
	}
	if !result.RevokeFailed {
		t.Fatalf("result = %#v, want revoke failure noted", result)
	}
	if _, err := cache.NewAccountRepository(db).Get(ctx, account.ID); !errors.Is(err, cache.ErrAccountNotFound) {
		t.Fatalf("account after remove = %v, want ErrAccountNotFound", err)
	}
}

func TestAccountRemoverLeavesOtherAccountsAndClientCredentials(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	store := NewSecretStore(newFakeKeyring())
	first := seedRemovableAccount(t, ctx, db, store, "first@example.com")
	second := seedRemovableAccount(t, ctx, db, store, "second@example.com")
	if err := store.StoreClientCredentials(ClientCredentials{ClientID: "client", ClientSecret: "secret"}); err != nil {
		t.Fatalf("store client credentials: %v", err)
	}

	remover := NewAccountRemover(db, store, "", "", nil)
	if _, err := remover.Remove(ctx, first.Email); err != nil {
		t.Fatalf("remove first account: %v", err)
	}
	if _, err := cache.NewAccountRepository(db).Get(ctx, second.ID); err != nil {
		t.Fatalf("second account missing: %v", err)
	}
	if _, err := store.OAuthToken(second.ID); err != nil {
		t.Fatalf("second token missing: %v", err)
	}
	if _, err := store.ClientCredentials(); err != nil {
		t.Fatalf("client credentials missing: %v", err)
	}
}

func seedRemovableAccount(t *testing.T, ctx context.Context, db *sqlx.DB, store SecretStore, email string) cache.Account {
	t.Helper()

	account, err := cache.NewAccountRepository(db).Create(ctx, cache.CreateAccountParams{Email: email})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := cache.NewLabelRepository(db).Upsert(ctx, cache.Label{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"}); err != nil {
		t.Fatalf("upsert label: %v", err)
	}
	if err := store.StoreOAuthToken(account.ID, &oauth2.Token{AccessToken: "access-" + email, RefreshToken: "refresh-" + email}); err != nil {
		t.Fatalf("store token: %v", err)
	}
	return account
}
