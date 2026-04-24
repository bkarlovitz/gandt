package main

import (
	"context"
	"errors"
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
	"github.com/bkarlovitz/gandt/internal/render"
	gandtsync "github.com/bkarlovitz/gandt/internal/sync"
	"github.com/bkarlovitz/gandt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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
		ui.WithAccountAdder(buildAccountAdder(paths, cfg)),
		ui.WithCredentialReplacer(buildCredentialReplacer()),
		ui.WithThreadLoader(buildThreadLoader(paths, cfg)),
		ui.WithManualRefresher(buildManualRefresher(paths, cfg)),
		ui.WithSyncCoordinator(buildSyncCoordinator(paths, cfg)),
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

func buildAccountAdder(paths config.Paths, cfg config.Config) ui.AccountAdder {
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

		backfiller := gandtsync.NewBackfiller(db, cfg, gmailClient)
		backfill, err := backfiller.Backfill(ctx, account)
		if err != nil {
			return ui.AccountAddResult{}, err
		}
		if _, err := backfiller.FetchBodies(ctx, account, backfill.BodyQueue); err != nil {
			return ui.AccountAddResult{}, err
		}
		if err := cache.NewAccountRepository(db).UpdateSyncMetadata(ctx, account.ID, account.HistoryID, time.Now().UTC()); err != nil {
			return ui.AccountAddResult{}, err
		}

		labels, err := cache.NewLabelRepository(db).List(ctx, account.ID)
		if err != nil {
			return ui.AccountAddResult{}, err
		}
		messagesByLabel, err := uiMessagesByLabel(ctx, cache.NewMessageRepository(db), account.ID, labels)
		if err != nil {
			return ui.AccountAddResult{}, err
		}
		return ui.AccountAddResult{
			Account:         account.Email,
			Labels:          uiLabels(cache.NewSyncPolicyRepository(db), account.ID, labels),
			MessagesByLabel: messagesByLabel,
		}, nil
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

func buildSyncCoordinator(paths config.Paths, cfg config.Config) ui.SyncCoordinator {
	return gandtsync.NewCoordinator(cfg, gandtsync.SyncRunnerFunc(func(ctx context.Context) (gandtsync.AccountSyncResult, error) {
		db, err := cache.Open(ctx, paths)
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		defer db.Close()
		if err := cache.Migrate(ctx, db); err != nil {
			return gandtsync.AccountSyncResult{}, err
		}

		accounts, err := cache.NewAccountRepository(db).List(ctx)
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		if len(accounts) == 0 {
			return gandtsync.AccountSyncResult{Status: "sync skipped: no accounts configured"}, nil
		}

		account := accounts[0]
		gmailClient, err := gmailClientForAccount(ctx, account.ID)
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		return gandtsync.NewDeltaSynchronizer(db, cfg, gmailClient).Sync(ctx, account)
	}))
}

func buildManualRefresher(paths config.Paths, cfg config.Config) ui.ManualRefresher {
	return ui.ManualRefresherFunc(func(request ui.RefreshRequest) (ui.RefreshResult, error) {
		ctx := context.Background()
		result, err := runOneAccountRefresh(ctx, paths, cfg, request)
		if err != nil {
			return ui.RefreshResult{}, err
		}
		return ui.RefreshResult{Summary: result.Status}, nil
	})
}

func runOneAccountRefresh(ctx context.Context, paths config.Paths, cfg config.Config, request ui.RefreshRequest) (gandtsync.AccountSyncResult, error) {
	db, err := cache.Open(ctx, paths)
	if err != nil {
		return gandtsync.AccountSyncResult{}, err
	}
	defer db.Close()
	if err := cache.Migrate(ctx, db); err != nil {
		return gandtsync.AccountSyncResult{}, err
	}

	accounts, err := cache.NewAccountRepository(db).List(ctx)
	if err != nil {
		return gandtsync.AccountSyncResult{}, err
	}
	if len(accounts) == 0 {
		return gandtsync.AccountSyncResult{Status: "sync skipped: no accounts configured"}, nil
	}
	account := accounts[0]
	if request.Account != "" {
		for _, candidate := range accounts {
			if candidate.Email == request.Account {
				account = candidate
				break
			}
		}
	}
	gmailClient, err := gmailClientForAccount(ctx, account.ID)
	if err != nil {
		return gandtsync.AccountSyncResult{}, err
	}

	if request.Kind == ui.RefreshRelistLabel {
		backfiller := gandtsync.NewBackfiller(db, cfg, gmailClient)
		backfill, err := backfiller.Backfill(ctx, account)
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		return gandtsync.AccountSyncResult{
			Backfill: backfill,
			Status:   fmt.Sprintf("refreshed %s", firstNonEmptyString(request.LabelName, request.LabelID, "label")),
		}, nil
	}
	return gandtsync.NewDeltaSynchronizer(db, cfg, gmailClient).Sync(ctx, account)
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

func buildThreadLoader(paths config.Paths, cfg config.Config) ui.ThreadLoader {
	return ui.ThreadLoaderFunc(func(request ui.ThreadLoadRequest) (ui.ThreadLoadResult, error) {
		if request.Message.ThreadID == "" {
			return ui.ThreadLoadResult{}, nil
		}

		ctx := context.Background()
		db, err := cache.Open(ctx, paths)
		if err != nil {
			return ui.ThreadLoadResult{}, err
		}
		defer db.Close()
		if err := cache.Migrate(ctx, db); err != nil {
			return ui.ThreadLoadResult{}, err
		}

		account, err := cache.NewAccountRepository(db).GetByEmail(ctx, request.Account)
		if err != nil {
			return ui.ThreadLoadResult{}, err
		}
		if result, ok, err := cachedThreadLoad(ctx, db, cfg, account.ID, request.Message); ok || err != nil {
			return result, err
		}

		gmailClient, err := gmailClientForAccount(ctx, account.ID)
		if err != nil {
			return ui.ThreadLoadResult{}, err
		}
		thread, err := gmailClient.GetThread(ctx, request.Message.ThreadID, gandtgmail.MessageFormatFull)
		if err != nil {
			return ui.ThreadLoadResult{}, offlineIfUnavailable(err)
		}
		result, err := streamedThreadLoad(ctx, db, cfg, account, gmailClient, request.Message, thread)
		if err != nil {
			return ui.ThreadLoadResult{}, offlineIfUnavailable(err)
		}
		return result, nil
	})
}

func gmailClientForAccount(ctx context.Context, accountID string) (*gandtgmail.Client, error) {
	secrets := auth.NewSecretStore(auth.SystemKeyring{})
	credentials, err := secrets.ClientCredentials()
	if err != nil {
		return nil, err
	}
	token, err := secrets.OAuthToken(accountID)
	if err != nil {
		return nil, err
	}

	oauthConfig := oauth2.Config{
		ClientID:     credentials.ClientID,
		ClientSecret: credentials.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       auth.GmailOAuthScopes,
	}
	httpClient := oauth2.NewClient(ctx, oauthConfig.TokenSource(ctx, token))
	return gandtgmail.NewClient(ctx, option.WithHTTPClient(httpClient))
}

func cachedThreadLoad(ctx context.Context, db *sqlx.DB, cfg config.Config, accountID string, message ui.Message) (ui.ThreadLoadResult, bool, error) {
	messages, err := cache.NewMessageRepository(db).ListByThread(ctx, accountID, message.ThreadID)
	if err != nil {
		return ui.ThreadLoadResult{}, false, err
	}
	if len(messages) == 0 {
		return ui.ThreadLoadResult{}, false, nil
	}

	attachments := cache.NewAttachmentRepository(db)
	result := ui.ThreadLoadResult{
		MessageID:   message.ID,
		ThreadID:    message.ThreadID,
		CacheState:  "cached",
		Attachments: nil,
	}
	selectedHasBody := false
	for _, cachedMessage := range messages {
		threadMessage, err := cachedUIThreadMessage(ctx, attachments, cfg, accountID, cachedMessage)
		if err != nil {
			return ui.ThreadLoadResult{}, false, err
		}
		if cachedMessage.ID == message.ID {
			result.Body = append([]string{}, threadMessage.Body...)
			result.Attachments = append([]ui.Attachment{}, threadMessage.Attachments...)
			selectedHasBody = len(threadMessage.Body) > 0
		}
		result.ThreadMessages = append(result.ThreadMessages, threadMessage)
	}
	if !selectedHasBody {
		return ui.ThreadLoadResult{}, false, nil
	}
	if len(result.Body) == 0 && len(result.ThreadMessages) > 0 {
		result.Body = append([]string{}, result.ThreadMessages[0].Body...)
		result.Attachments = append([]ui.Attachment{}, result.ThreadMessages[0].Attachments...)
	}
	return result, true, nil
}

func streamedThreadLoad(ctx context.Context, db *sqlx.DB, cfg config.Config, account cache.Account, client gandtgmail.MessageReader, selected ui.Message, thread gandtgmail.Thread) (ui.ThreadLoadResult, error) {
	backfiller := gandtsync.NewBackfiller(db, cfg, client)
	evaluator := gandtsync.NewPolicyEvaluator(db, cfg)
	result := ui.ThreadLoadResult{
		MessageID:  selected.ID,
		ThreadID:   selected.ThreadID,
		CacheState: "streamed",
	}

	for _, message := range thread.Messages {
		message.ThreadID = firstNonEmptyString(message.ThreadID, thread.ID, selected.ThreadID)
		threadMessage, err := gmailUIThreadMessage(cfg, message)
		if err != nil {
			return ui.ThreadLoadResult{}, err
		}
		decision, err := evaluator.Evaluate(ctx, gandtsync.MessageContext{
			AccountID:    account.ID,
			AccountEmail: account.Email,
			From:         gmailHeaderValue(message.Headers, "From"),
			LabelIDs:     message.LabelIDs,
		})
		if err != nil {
			return ui.ThreadLoadResult{}, err
		}

		cacheState := "streamed"
		switch {
		case decision.Excluded:
			cacheState = "excluded"
		case decision.Depth == config.CacheDepthBody || decision.Depth == config.CacheDepthFull:
			if _, err := backfiller.PersistFullMessage(ctx, account, message); err != nil {
				return ui.ThreadLoadResult{}, err
			}
			cacheState = "cached"
		}

		if message.ID == selected.ID {
			result.Body = append([]string{}, threadMessage.Body...)
			result.Attachments = append([]ui.Attachment{}, threadMessage.Attachments...)
			result.CacheState = cacheState
		}
		result.ThreadMessages = append(result.ThreadMessages, threadMessage)
	}
	if len(result.Body) == 0 && len(result.ThreadMessages) > 0 {
		result.Body = append([]string{}, result.ThreadMessages[0].Body...)
		result.Attachments = append([]ui.Attachment{}, result.ThreadMessages[0].Attachments...)
	}
	return result, nil
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

func cachedUIThreadMessage(ctx context.Context, attachments cache.AttachmentRepository, cfg config.Config, accountID string, message cache.Message) (ui.ThreadMessage, error) {
	body, err := cachedBodyLines(cfg, message)
	if err != nil {
		return ui.ThreadMessage{}, err
	}
	cachedAttachments, err := attachments.ListForMessage(ctx, accountID, message.ID)
	if err != nil {
		return ui.ThreadMessage{}, err
	}
	from, address := displaySender(message.FromAddr)
	return ui.ThreadMessage{
		ID:          message.ID,
		From:        from,
		Address:     address,
		Date:        displayDate(message.InternalDate, message.Date),
		Body:        body,
		Attachments: uiAttachments(cachedAttachments),
	}, nil
}

func cachedBodyLines(cfg config.Config, message cache.Message) ([]string, error) {
	if message.BodyPlain != nil {
		return bodyLines(*message.BodyPlain), nil
	}
	if message.BodyHTML == nil {
		return nil, nil
	}
	text, err := render.HTMLToText(*message.BodyHTML, render.HTMLRenderOptions{URLFootnotes: cfg.UI.RenderURLFootnotes})
	if err != nil {
		return nil, err
	}
	return bodyLines(text), nil
}

func gmailUIThreadMessage(cfg config.Config, message gandtgmail.Message) (ui.ThreadMessage, error) {
	extracted, err := gandtgmail.ExtractBody(message, gandtgmail.BodyExtractionOptions{KeepHTML: true})
	if err != nil {
		return ui.ThreadMessage{}, err
	}
	body, err := extractedBodyLines(cfg, extracted)
	if err != nil {
		return ui.ThreadMessage{}, err
	}
	from, address := displaySender(gmailHeaderValue(message.Headers, "From"))
	return ui.ThreadMessage{
		ID:          message.ID,
		From:        from,
		Address:     address,
		Date:        displayDate(timePtr(message.InternalDate)),
		Body:        body,
		Attachments: uiMIMEAttachments(extracted.Attachments),
	}, nil
}

func extractedBodyLines(cfg config.Config, extracted gandtgmail.ExtractedBody) ([]string, error) {
	if extracted.Plain != nil {
		return bodyLines(*extracted.Plain), nil
	}
	if extracted.FallbackHTML == nil {
		return nil, nil
	}
	text, err := render.HTMLToText(*extracted.FallbackHTML, render.HTMLRenderOptions{URLFootnotes: cfg.UI.RenderURLFootnotes})
	if err != nil {
		return nil, err
	}
	return bodyLines(text), nil
}

func bodyLines(body string) []string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func uiAttachments(attachments []cache.Attachment) []ui.Attachment {
	out := make([]ui.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		out = append(out, ui.Attachment{
			Name: firstNonEmptyString(attachment.Filename, "unnamed"),
			Size: humanBytes(attachment.SizeBytes),
		})
	}
	return out
}

func uiMIMEAttachments(attachments []gandtgmail.MIMEAttachment) []ui.Attachment {
	out := make([]ui.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		out = append(out, ui.Attachment{
			Name: firstNonEmptyString(attachment.Filename, "unnamed"),
			Size: humanBytes(attachment.Size),
		})
	}
	return out
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

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func gmailHeaderValue(headers []gandtgmail.MessageHeader, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func offlineIfUnavailable(err error) error {
	if errors.Is(err, gandtgmail.ErrUnavailable) {
		return ui.MarkOffline(err)
	}
	return err
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func humanBytes(size int) string {
	if size < 0 {
		size = 0
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/unit)
}
