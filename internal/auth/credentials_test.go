package auth

import (
	"context"
	"errors"
	"testing"
)

func TestCredentialSetupPromptsAndPersistsOnFirstRun(t *testing.T) {
	fake := newFakeKeyring()
	setup := NewCredentialSetup(NewSecretStore(fake))
	prompt := &fakeCredentialPrompt{
		credentials: ClientCredentials{
			ClientID:     " client-id ",
			ClientSecret: " client-secret ",
		},
	}

	credentials, stored, err := setup.EnsureClientCredentials(context.Background(), prompt)
	if err != nil {
		t.Fatalf("ensure credentials: %v", err)
	}
	if !stored {
		t.Fatal("expected first-run credentials to be stored")
	}
	if prompt.calls != 1 {
		t.Fatalf("prompt calls = %d, want 1", prompt.calls)
	}
	if credentials.ClientID != "client-id" || credentials.ClientSecret != "client-secret" {
		t.Fatalf("credentials = %#v, want trimmed values", credentials)
	}

	loaded, err := NewSecretStore(fake).ClientCredentials()
	if err != nil {
		t.Fatalf("load stored credentials: %v", err)
	}
	if loaded != credentials {
		t.Fatalf("stored credentials = %#v, want %#v", loaded, credentials)
	}
}

func TestCredentialSetupUsesExistingCredentialsWithoutPrompt(t *testing.T) {
	fake := newFakeKeyring()
	store := NewSecretStore(fake)
	existing := ClientCredentials{ClientID: "client-id", ClientSecret: "client-secret"}
	if err := store.StoreClientCredentials(existing); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}

	prompt := &fakeCredentialPrompt{}
	credentials, stored, err := NewCredentialSetup(store).EnsureClientCredentials(context.Background(), prompt)
	if err != nil {
		t.Fatalf("ensure credentials: %v", err)
	}
	if stored {
		t.Fatal("did not expect existing credentials to be stored again")
	}
	if prompt.calls != 0 {
		t.Fatalf("prompt calls = %d, want 0", prompt.calls)
	}
	if credentials != existing {
		t.Fatalf("credentials = %#v, want %#v", credentials, existing)
	}
}

func TestCredentialSetupRejectsInvalidCredentials(t *testing.T) {
	tests := map[string]ClientCredentials{
		"missing id":     {ClientSecret: "client-secret"},
		"missing secret": {ClientID: "client-id"},
	}

	for name, credentials := range tests {
		t.Run(name, func(t *testing.T) {
			setup := NewCredentialSetup(NewSecretStore(newFakeKeyring()))
			prompt := &fakeCredentialPrompt{credentials: credentials}
			_, _, err := setup.EnsureClientCredentials(context.Background(), prompt)
			if !errors.Is(err, ErrInvalidClientCredentials) {
				t.Fatalf("error = %v, want ErrInvalidClientCredentials", err)
			}
		})
	}
}

func TestCredentialSetupReplacementRequiresConfirmation(t *testing.T) {
	fake := newFakeKeyring()
	store := NewSecretStore(fake)
	if err := store.StoreClientCredentials(ClientCredentials{ClientID: "old-id", ClientSecret: "old-secret"}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}

	setup := NewCredentialSetup(store)
	prompt := &fakeCredentialPrompt{
		credentials: ClientCredentials{ClientID: "new-id", ClientSecret: "new-secret"},
	}
	if _, err := setup.ReplaceClientCredentials(context.Background(), prompt, false); !errors.Is(err, ErrCredentialReplacementCanceled) {
		t.Fatalf("replace without confirmation error = %v, want ErrCredentialReplacementCanceled", err)
	}
	if prompt.calls != 0 {
		t.Fatalf("prompt calls = %d, want 0", prompt.calls)
	}

	credentials, err := setup.ReplaceClientCredentials(context.Background(), prompt, true)
	if err != nil {
		t.Fatalf("replace credentials: %v", err)
	}
	if credentials.ClientID != "new-id" || credentials.ClientSecret != "new-secret" {
		t.Fatalf("credentials = %#v, want new values", credentials)
	}
	loaded, err := store.ClientCredentials()
	if err != nil {
		t.Fatalf("load replaced credentials: %v", err)
	}
	if loaded != credentials {
		t.Fatalf("stored credentials = %#v, want %#v", loaded, credentials)
	}
}

type fakeCredentialPrompt struct {
	credentials ClientCredentials
	err         error
	calls       int
}

func (p *fakeCredentialPrompt) PromptClientCredentials(context.Context) (ClientCredentials, error) {
	p.calls++
	if p.err != nil {
		return ClientCredentials{}, p.err
	}
	return p.credentials, nil
}
