package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

type TokenRevoker interface {
	RevokeToken(context.Context, *oauth2.Token) error
}

type TokenRevokerFunc func(context.Context, *oauth2.Token) error

func (fn TokenRevokerFunc) RevokeToken(ctx context.Context, token *oauth2.Token) error {
	return fn(ctx, token)
}

type GoogleTokenRevoker struct {
	HTTPClient *http.Client
}

func (r GoogleTokenRevoker) RevokeToken(ctx context.Context, token *oauth2.Token) error {
	if token == nil {
		return nil
	}
	value := firstNonEmpty(token.RefreshToken, token.AccessToken)
	if value == "" {
		return nil
	}
	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/revoke", strings.NewReader(url.Values{"token": {value}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("google token revoke returned %s", resp.Status)
	}
	return nil
}

type RemoveAccountResult struct {
	Account      cache.Account
	RevokeFailed bool
}

type AccountRemover struct {
	accounts       cache.AccountRepository
	secrets        SecretStore
	registryPath   string
	attachmentRoot string
	revoker        TokenRevoker
}

func NewAccountRemover(db *sqlx.DB, secrets SecretStore, registryPath string, attachmentRoot string, revoker TokenRevoker) AccountRemover {
	return AccountRemover{
		accounts:       cache.NewAccountRepository(db),
		secrets:        secrets,
		registryPath:   registryPath,
		attachmentRoot: attachmentRoot,
		revoker:        revoker,
	}
}

func (r AccountRemover) Remove(ctx context.Context, accountRef string) (RemoveAccountResult, error) {
	account, err := r.findAccount(ctx, accountRef)
	if err != nil {
		return RemoveAccountResult{}, err
	}

	token, tokenErr := r.secrets.OAuthToken(account.ID)
	revokeFailed := false
	if tokenErr == nil && r.revoker != nil {
		if err := r.revoker.RevokeToken(ctx, token); err != nil {
			revokeFailed = true
		}
	}

	if err := r.secrets.DeleteOAuthToken(account.ID); err != nil && !errors.Is(err, ErrSecretNotFound) {
		return RemoveAccountResult{}, err
	}
	if err := r.accounts.Delete(ctx, account.ID); err != nil {
		return RemoveAccountResult{}, err
	}
	if err := r.removeAttachments(account.ID); err != nil {
		return RemoveAccountResult{}, err
	}
	if err := WriteAccountRegistry(ctx, r.accounts, r.registryPath); err != nil {
		return RemoveAccountResult{}, err
	}

	return RemoveAccountResult{Account: account, RevokeFailed: revokeFailed}, nil
}

func (r AccountRemover) findAccount(ctx context.Context, accountRef string) (cache.Account, error) {
	ref := strings.TrimSpace(accountRef)
	if ref == "" {
		return cache.Account{}, errors.New("account is required")
	}
	if account, err := r.accounts.Get(ctx, ref); err == nil {
		return account, nil
	} else if !errors.Is(err, cache.ErrAccountNotFound) {
		return cache.Account{}, err
	}
	return r.accounts.GetByEmail(ctx, ref)
}

func (r AccountRemover) removeAttachments(accountID string) error {
	if strings.TrimSpace(r.attachmentRoot) == "" {
		return nil
	}
	if err := os.RemoveAll(filepath.Join(r.attachmentRoot, accountID)); err != nil {
		return fmt.Errorf("remove account attachments: %w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
