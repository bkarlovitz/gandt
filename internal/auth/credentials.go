package auth

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidClientCredentials      = errors.New("invalid client credentials")
	ErrCredentialReplacementCanceled = errors.New("client credential replacement canceled")
)

type CredentialPrompt interface {
	PromptClientCredentials(context.Context) (ClientCredentials, error)
}

type CredentialSetup struct {
	store SecretStore
}

func NewCredentialSetup(store SecretStore) CredentialSetup {
	return CredentialSetup{store: store}
}

func ValidateClientCredentials(credentials ClientCredentials) error {
	if strings.TrimSpace(credentials.ClientID) == "" {
		return ErrInvalidClientCredentials
	}
	if strings.TrimSpace(credentials.ClientSecret) == "" {
		return ErrInvalidClientCredentials
	}
	return nil
}

func (setup CredentialSetup) EnsureClientCredentials(ctx context.Context, prompt CredentialPrompt) (ClientCredentials, bool, error) {
	credentials, err := setup.store.ClientCredentials()
	if err == nil {
		return credentials, false, nil
	}
	if !errors.Is(err, ErrSecretNotFound) {
		return ClientCredentials{}, false, err
	}
	if prompt == nil {
		return ClientCredentials{}, false, ErrSecretNotFound
	}

	credentials, err = prompt.PromptClientCredentials(ctx)
	if err != nil {
		return ClientCredentials{}, false, err
	}
	credentials = normalizeClientCredentials(credentials)
	if err := ValidateClientCredentials(credentials); err != nil {
		return ClientCredentials{}, false, err
	}
	if err := setup.store.StoreClientCredentials(credentials); err != nil {
		return ClientCredentials{}, false, err
	}
	return credentials, true, nil
}

func (setup CredentialSetup) ReplaceClientCredentials(ctx context.Context, prompt CredentialPrompt, confirmed bool) (ClientCredentials, error) {
	if !confirmed {
		return ClientCredentials{}, ErrCredentialReplacementCanceled
	}
	if prompt == nil {
		return ClientCredentials{}, ErrInvalidClientCredentials
	}

	credentials, err := prompt.PromptClientCredentials(ctx)
	if err != nil {
		return ClientCredentials{}, err
	}
	credentials = normalizeClientCredentials(credentials)
	if err := ValidateClientCredentials(credentials); err != nil {
		return ClientCredentials{}, err
	}
	if err := setup.store.StoreClientCredentials(credentials); err != nil {
		return ClientCredentials{}, err
	}
	return credentials, nil
}

func normalizeClientCredentials(credentials ClientCredentials) ClientCredentials {
	return ClientCredentials{
		ClientID:     strings.TrimSpace(credentials.ClientID),
		ClientSecret: strings.TrimSpace(credentials.ClientSecret),
	}
}
