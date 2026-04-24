package auth

import (
	"context"
	"errors"
	"hash/fnv"
	"strings"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

type GmailBootstrapClient interface {
	Profile(context.Context) (gmail.Profile, error)
	Labels(context.Context) ([]gmail.Label, error)
}

type AccountBootstrapper struct {
	accounts     cache.AccountRepository
	labels       cache.LabelRepository
	secrets      SecretStore
	registryPath string
}

func NewAccountBootstrapper(db *sqlx.DB, secrets SecretStore) AccountBootstrapper {
	return AccountBootstrapper{
		accounts: cache.NewAccountRepository(db),
		labels:   cache.NewLabelRepository(db),
		secrets:  secrets,
	}
}

func NewAccountBootstrapperWithRegistry(db *sqlx.DB, secrets SecretStore, registryPath string) AccountBootstrapper {
	bootstrapper := NewAccountBootstrapper(db, secrets)
	bootstrapper.registryPath = registryPath
	return bootstrapper
}

func (b AccountBootstrapper) Bootstrap(ctx context.Context, client GmailBootstrapClient, token *oauth2.Token, configuredColor string) (cache.Account, error) {
	if client == nil {
		return cache.Account{}, errors.New("gmail client is required")
	}
	if token == nil {
		return cache.Account{}, errors.New("oauth token is required")
	}

	profile, err := client.Profile(ctx)
	if err != nil {
		return cache.Account{}, err
	}
	labels, err := client.Labels(ctx)
	if err != nil {
		return cache.Account{}, err
	}

	color := strings.TrimSpace(configuredColor)
	if color == "" {
		color = DeterministicAccountColor(profile.EmailAddress)
	}
	account, err := b.accounts.Create(ctx, cache.CreateAccountParams{
		Email:     profile.EmailAddress,
		HistoryID: profile.HistoryID,
		Color:     color,
	})
	if err != nil {
		return cache.Account{}, err
	}

	if err := b.secrets.StoreOAuthToken(account.ID, token); err != nil {
		_ = b.accounts.Delete(ctx, account.ID)
		return cache.Account{}, err
	}
	for _, label := range labels {
		if err := b.labels.Upsert(ctx, cache.Label{
			AccountID: account.ID,
			ID:        label.ID,
			Name:      label.Name,
			Type:      label.Type,
			Unread:    label.Unread,
			Total:     label.Total,
			ColorBG:   label.ColorBG,
			ColorFG:   label.ColorFG,
		}); err != nil {
			_ = b.secrets.DeleteOAuthToken(account.ID)
			_ = b.accounts.Delete(ctx, account.ID)
			return cache.Account{}, err
		}
	}

	if err := WriteAccountRegistry(ctx, b.accounts, b.registryPath); err != nil {
		_ = b.secrets.DeleteOAuthToken(account.ID)
		_ = b.accounts.Delete(ctx, account.ID)
		return cache.Account{}, err
	}

	return account, nil
}

func DeterministicAccountColor(email string) string {
	palette := []string{
		"#4285f4",
		"#0f9d58",
		"#db4437",
		"#f4b400",
		"#ab47bc",
		"#00acc1",
		"#ff7043",
		"#5c6bc0",
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return palette[int(hash.Sum32())%len(palette)]
}
