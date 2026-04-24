package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	keyring "github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	KeyringService       = "com.<owner>.gandt"
	clientCredentialsKey = "client-credentials"
	oauthTokenKeyPrefix  = "oauth-token:"
)

var ErrSecretNotFound = errors.New("secret not found")

type ClientCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Keyring interface {
	Set(service, user, password string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

type SystemKeyring struct{}

func (SystemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (SystemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (SystemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type SecretStore struct {
	keyring Keyring
}

func NewSecretStore(keyring Keyring) SecretStore {
	return SecretStore{keyring: keyring}
}

func (s SecretStore) StoreClientCredentials(credentials ClientCredentials) error {
	payload, err := json.Marshal(credentials)
	if err != nil {
		return redacted("encode client credentials", err)
	}
	if err := s.keyring.Set(KeyringService, clientCredentialsKey, string(payload)); err != nil {
		return redacted("store client credentials", err)
	}
	return nil
}

func (s SecretStore) ClientCredentials() (ClientCredentials, error) {
	payload, err := s.keyring.Get(KeyringService, clientCredentialsKey)
	if err != nil {
		return ClientCredentials{}, mapSecretError("load client credentials", err)
	}

	var credentials ClientCredentials
	if err := json.Unmarshal([]byte(payload), &credentials); err != nil {
		return ClientCredentials{}, redacted("decode client credentials", err)
	}
	return credentials, nil
}

func (s SecretStore) DeleteClientCredentials() error {
	if err := s.keyring.Delete(KeyringService, clientCredentialsKey); err != nil {
		return mapSecretError("delete client credentials", err)
	}
	return nil
}

func (s SecretStore) StoreOAuthToken(accountID string, token *oauth2.Token) error {
	if strings.TrimSpace(accountID) == "" {
		return errors.New("account id is required")
	}
	if token == nil {
		return errors.New("oauth token is required")
	}

	payload, err := json.Marshal(token)
	if err != nil {
		return redacted("encode oauth token", err)
	}
	if err := s.keyring.Set(KeyringService, oauthTokenKey(accountID), string(payload)); err != nil {
		return redacted("store oauth token", err)
	}
	return nil
}

func (s SecretStore) OAuthToken(accountID string) (*oauth2.Token, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, errors.New("account id is required")
	}

	payload, err := s.keyring.Get(KeyringService, oauthTokenKey(accountID))
	if err != nil {
		return nil, mapSecretError("load oauth token", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(payload), &token); err != nil {
		return nil, redacted("decode oauth token", err)
	}
	return &token, nil
}

func (s SecretStore) DeleteOAuthToken(accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return errors.New("account id is required")
	}
	if err := s.keyring.Delete(KeyringService, oauthTokenKey(accountID)); err != nil {
		return mapSecretError("delete oauth token", err)
	}
	return nil
}

func oauthTokenKey(accountID string) string {
	return oauthTokenKeyPrefix + accountID
}

func mapSecretError(operation string, err error) error {
	if errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("%s: %w", operation, ErrSecretNotFound)
	}
	return redacted(operation, err)
}

func redacted(operation string, err error) error {
	return redactedError{operation: operation, err: err}
}

type redactedError struct {
	operation string
	err       error
}

func (err redactedError) Error() string {
	return err.operation + " failed"
}

func (err redactedError) Unwrap() error {
	return err.err
}
