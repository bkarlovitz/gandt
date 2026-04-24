package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/bkarlovitz/gandt/internal/auth"
	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	gandtgmail "github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/bkarlovitz/gandt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gandt: resolve paths: %v\n", err)
		os.Exit(1)
	}
	if err := config.EnsureDirs(paths); err != nil {
		fmt.Fprintf(os.Stderr, "gandt: create data directories: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gandt: load config: %v\n", err)
		os.Exit(1)
	}
	logFile, err := config.InitFileLogger(paths, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gandt: initialize log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	uiOptions := []ui.Option{ui.WithAccountAdder(buildAccountAdder(paths))}
	if mailbox, ok := loadInitialMailbox(paths); ok {
		uiOptions = append(uiOptions, ui.WithMailbox(mailbox))
	}
	program := tea.NewProgram(ui.New(cfg, uiOptions...), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "gandt: %v\n", err)
		os.Exit(1)
	}
}

func buildAccountAdder(paths config.Paths) ui.AccountAdder {
	return ui.AccountAdderFunc(func() (ui.AccountAddResult, error) {
		ctx := context.Background()
		db, err := cache.Open(ctx, paths)
		if err != nil {
			return ui.AccountAddResult{}, err
		}
		defer db.Close()
		if err := cache.Migrate(ctx, db); err != nil {
			return ui.AccountAddResult{}, err
		}

		secrets := auth.NewSecretStore(auth.SystemKeyring{})
		credentials, err := secrets.ClientCredentials()
		if errors.Is(err, auth.ErrSecretNotFound) {
			return ui.AccountAddResult{}, errors.New("OAuth client credentials are not configured")
		}
		if err != nil {
			return ui.AccountAddResult{}, err
		}

		token, err := auth.RunLoopbackOAuth(ctx, credentials, auth.LoopbackOAuthOptions{})
		if err != nil {
			return ui.AccountAddResult{}, err
		}

		httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
		gmailClient, err := gandtgmail.NewClient(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			return ui.AccountAddResult{}, err
		}

		account, err := auth.NewAccountBootstrapper(db, secrets).Bootstrap(ctx, gmailClient, token, "")
		if err != nil {
			return ui.AccountAddResult{}, err
		}

		labels, err := cache.NewLabelRepository(db).List(ctx, account.ID)
		if err != nil {
			return ui.AccountAddResult{}, err
		}
		return ui.AccountAddResult{Account: account.Email, Labels: uiLabels(cache.NewSyncPolicyRepository(db), account.ID, labels)}, nil
	})
}

func loadInitialMailbox(paths config.Paths) (ui.Mailbox, bool) {
	ctx := context.Background()
	db, err := cache.Open(ctx, paths)
	if err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	defer db.Close()
	if err := cache.Migrate(ctx, db); err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}

	accounts, err := cache.NewAccountRepository(db).List(ctx)
	if err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	if len(accounts) == 0 {
		return ui.NoAccountMailbox(), true
	}

	account := accounts[0]
	labels, err := cache.NewLabelRepository(db).List(ctx, account.ID)
	if err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	return ui.RealAccountMailbox(account.Email, uiLabels(cache.NewSyncPolicyRepository(db), account.ID, labels)), true
}

func uiLabels(policies cache.SyncPolicyRepository, accountID string, labels []cache.Label) []ui.Label {
	out := make([]ui.Label, 0, len(labels))
	for _, label := range labels {
		depth := ""
		if policy, err := policies.EffectiveForLabel(context.Background(), accountID, label.ID); err == nil {
			depth = policy.Depth
		}
		out = append(out, ui.Label{
			Name:       label.Name,
			Unread:     label.Unread,
			System:     label.Type == "system",
			CacheDepth: depth,
		})
	}
	return out
}
