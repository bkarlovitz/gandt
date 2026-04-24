package auth

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	keyring "github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func TestSecretStoreClientCredentials(t *testing.T) {
	fake := newFakeKeyring()
	store := NewSecretStore(fake)

	credentials := ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}
	if err := store.StoreClientCredentials(credentials); err != nil {
		t.Fatalf("store credentials: %v", err)
	}

	if fake.lastService != KeyringService || fake.lastUser != clientCredentialsKey {
		t.Fatalf("stored at service=%q user=%q", fake.lastService, fake.lastUser)
	}

	var stored ClientCredentials
	if err := json.Unmarshal([]byte(fake.secrets[secretKey(KeyringService, clientCredentialsKey)]), &stored); err != nil {
		t.Fatalf("decode stored credentials: %v", err)
	}
	if stored != credentials {
		t.Fatalf("stored credentials = %#v, want %#v", stored, credentials)
	}

	got, err := store.ClientCredentials()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if got != credentials {
		t.Fatalf("credentials = %#v, want %#v", got, credentials)
	}
}

func TestSecretStoreOAuthToken(t *testing.T) {
	fake := newFakeKeyring()
	store := NewSecretStore(fake)
	token := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 4, 24, 13, 30, 0, 0, time.UTC),
	}

	if err := store.StoreOAuthToken("account-1", token); err != nil {
		t.Fatalf("store token: %v", err)
	}
	if fake.lastService != KeyringService || fake.lastUser != "oauth-token:account-1" {
		t.Fatalf("stored at service=%q user=%q", fake.lastService, fake.lastUser)
	}

	got, err := store.OAuthToken("account-1")
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if got.AccessToken != token.AccessToken || got.RefreshToken != token.RefreshToken || !got.Expiry.Equal(token.Expiry) {
		t.Fatalf("token = %#v, want %#v", got, token)
	}

	if err := store.DeleteOAuthToken("account-1"); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	if _, err := store.OAuthToken("account-1"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("load deleted token error = %v, want ErrSecretNotFound", err)
	}
}

func TestSecretStoreErrorsAreRedacted(t *testing.T) {
	fake := newFakeKeyring()
	fake.setErr = errors.New("backend rejected client-secret access-token")
	store := NewSecretStore(fake)

	err := store.StoreClientCredentials(ClientCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err == nil {
		t.Fatal("expected store credentials error")
	}
	if strings.Contains(err.Error(), "client-secret") || strings.Contains(err.Error(), "access-token") {
		t.Fatalf("error leaked secret value: %v", err)
	}
	if !errors.Is(err, fake.setErr) {
		t.Fatalf("error does not unwrap backend error")
	}

	err = store.StoreOAuthToken("account-1", &oauth2.Token{AccessToken: "access-token"})
	if err == nil {
		t.Fatal("expected store token error")
	}
	if strings.Contains(err.Error(), "client-secret") || strings.Contains(err.Error(), "access-token") {
		t.Fatalf("error leaked token value: %v", err)
	}
}

type fakeKeyring struct {
	secrets     map[string]string
	lastService string
	lastUser    string
	setErr      error
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{secrets: map[string]string{}}
}

func (f *fakeKeyring) Set(service, user, password string) error {
	f.lastService = service
	f.lastUser = user
	if f.setErr != nil {
		return f.setErr
	}
	f.secrets[secretKey(service, user)] = password
	return nil
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	value, ok := f.secrets[secretKey(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	key := secretKey(service, user)
	if _, ok := f.secrets[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.secrets, key)
	return nil
}

func secretKey(service, user string) string {
	return service + "\x00" + user
}
