package main

import (
	"context"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

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

	uiOptions := []ui.Option{
		ui.WithAccountAdder(buildAccountAdder(paths)),
		ui.WithCredentialReplacer(buildCredentialReplacer()),
	}
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
		credentials, _, err := auth.NewCredentialSetup(secrets).EnsureClientCredentials(ctx, auth.HuhCredentialPrompt{})
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

func buildCredentialReplacer() ui.CredentialReplacer {
	return ui.CredentialReplacerFunc(func() error {
		ctx := context.Background()
		secrets := auth.NewSecretStore(auth.SystemKeyring{})
		setup := auth.NewCredentialSetup(secrets)
		prompt := auth.HuhCredentialPrompt{}

		confirmed, err := prompt.ConfirmClientCredentialReplacement(ctx)
		if err != nil {
			return err
		}
		_, err = setup.ReplaceClientCredentials(ctx, prompt, confirmed)
		return err
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
	labelsForUI := uiLabels(cache.NewSyncPolicyRepository(db), account.ID, labels)
	messagesByLabel, err := uiMessagesByLabel(ctx, cache.NewMessageRepository(db), account.ID, labels)
	if err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	return ui.RealAccountMailbox(account.Email, labelsForUI, messagesByLabel), true
}

func uiLabels(policies cache.SyncPolicyRepository, accountID string, labels []cache.Label) []ui.Label {
	out := make([]ui.Label, 0, len(labels))
	for _, label := range labels {
		depth := ""
		if policy, err := policies.EffectiveForLabel(context.Background(), accountID, label.ID); err == nil {
			depth = policy.Depth
		}
		out = append(out, ui.Label{
			ID:         label.ID,
			Name:       label.Name,
			Unread:     label.Unread,
			System:     label.Type == "system",
			CacheDepth: depth,
		})
	}
	return out
}

func uiMessagesByLabel(ctx context.Context, messages cache.MessageRepository, accountID string, labels []cache.Label) (map[string][]ui.Message, error) {
	out := map[string][]ui.Message{}
	for _, label := range labels {
		summaries, err := messages.ListSummariesByLabel(ctx, accountID, label.ID, 5000)
		if err != nil {
			return nil, err
		}
		for _, summary := range summaries {
			out[label.ID] = append(out[label.ID], uiMessage(summary))
		}
	}
	return out, nil
}

func uiMessage(summary cache.MessageSummary) ui.Message {
	cacheState := "metadata"
	if summary.BodyCached {
		cacheState = "cached"
	}
	from, address := displaySender(summary.FromAddr)
	return ui.Message{
		ID:              summary.ID,
		ThreadID:        summary.ThreadID,
		From:            from,
		Address:         address,
		Subject:         summary.Subject,
		Date:            displayDate(summary.InternalDate, summary.Date),
		Snippet:         summary.Snippet,
		Unread:          summary.Unread,
		ThreadCount:     summary.ThreadCount,
		CacheState:      cacheState,
		AttachmentCount: summary.AttachmentCount,
	}
}

func displaySender(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(unknown)", ""
	}
	address, err := mail.ParseAddress(value)
	if err != nil {
		return value, value
	}
	if strings.TrimSpace(address.Name) != "" {
		return address.Name, address.Address
	}
	return address.Address, address.Address
}

func displayDate(values ...*time.Time) string {
	for _, value := range values {
		if value != nil && !value.IsZero() {
			return value.Local().Format("Jan 02")
		}
	}
	return ""
}
