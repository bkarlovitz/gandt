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
	charmlog "github.com/charmbracelet/log"
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
		ui.WithTriageActor(buildTriageActor(paths)),
		ui.WithCacheDashboardLoader(buildCacheDashboardLoader(paths, cfg)),
		ui.WithCachePolicyStore(buildCachePolicyStore(paths, cfg)),
		ui.WithCacheExclusionStore(buildCacheExclusionStore(paths)),
		ui.WithCachePurgeStore(buildCachePurgeStore(paths)),
		ui.WithCacheWipeStore(buildCacheWipeStore(paths)),
		ui.WithSyncCoordinator(buildSyncCoordinator(paths, cfg)),
	}
	if mailbox, ok := loadInitialMailbox(paths, cfg); ok {
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
			Labels:          uiLabels(ctx, db, cfg, account, labels),
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
	retentionSchedule := gandtsync.NewRetentionSchedule()
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
		now := time.Now().UTC()
		if retentionSchedule.ShouldRun(account.ID, now) {
			if _, err := gandtsync.NewRetentionSweeper(db, cfg).Sweep(ctx, account, now); err != nil {
				return gandtsync.AccountSyncResult{}, err
			}
		}
		gmailClient, err := gmailClientForAccount(ctx, account.ID)
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		return gandtsync.NewDeltaSynchronizer(db, cfg, gmailClient, gandtsync.WithLogger(charmSyncLogger{})).Sync(ctx, account)
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

func buildCacheDashboardLoader(paths config.Paths, cfg config.Config) ui.CacheDashboardLoader {
	return ui.CacheDashboardLoaderFunc(func() (ui.CacheDashboard, error) {
		ctx := context.Background()
		db, err := cache.Open(ctx, paths)
		if err != nil {
			return ui.CacheDashboard{}, err
		}
		defer db.Close()
		if err := cache.Migrate(ctx, db); err != nil {
			return ui.CacheDashboard{}, err
		}

		stats, err := cache.NewCacheStatsService(db).Summary(ctx, time.Now().UTC())
		if err != nil {
			return ui.CacheDashboard{}, err
		}
		return uiCacheDashboard(ctx, db, cfg, stats), nil
	})
}

func uiCacheDashboard(ctx context.Context, db *sqlx.DB, cfg config.Config, stats cache.CacheStats) ui.CacheDashboard {
	dashboard := ui.CacheDashboard{
		GeneratedAt:           stats.GeneratedAt,
		SQLiteBytes:           stats.Total.SQLiteBytes,
		TotalBytes:            stats.Total.TotalBytes,
		MessageCount:          stats.Total.MessageCount,
		BodyCount:             stats.Total.BodyCount,
		AttachmentCount:       stats.Attachments.AttachmentCount,
		CachedAttachmentCount: stats.Attachments.CachedFileCount,
		MessageBytes:          stats.Total.MessageBytes,
		BodyBytes:             stats.Total.BodyBytes,
		AttachmentBytes:       stats.Total.AttachmentBytes,
		FTSBytes:              stats.FTS.Bytes,
		FTSRows:               stats.FTS.RowCount,
	}

	accountEmails := map[string]string{}
	for _, account := range stats.Accounts {
		accountEmails[account.AccountID] = account.Email
		dashboard.Accounts = append(dashboard.Accounts, ui.CacheDashboardAccount{
			Email:           account.Email,
			MessageCount:    account.MessageCount,
			BodyCount:       account.BodyCount,
			AttachmentCount: account.AttachmentCount,
			TotalBytes:      account.TotalBytes,
		})
	}

	evaluator := gandtsync.NewPolicyEvaluator(db, cfg)
	for _, label := range stats.Labels {
		depth := ""
		if policy, err := evaluator.EffectiveForLabel(ctx, label.AccountID, accountEmails[label.AccountID], label.LabelID); err == nil {
			depth = string(policy.Depth)
		}
		dashboard.Labels = append(dashboard.Labels, ui.CacheDashboardLabel{
			AccountEmail:    accountEmails[label.AccountID],
			LabelID:         label.LabelID,
			LabelName:       label.LabelName,
			CacheDepth:      depth,
			MessageCount:    label.MessageCount,
			BodyCount:       label.BodyCount,
			AttachmentCount: label.AttachmentCount,
			AttachmentBytes: label.AttachmentBytes,
			TotalBytes:      label.TotalBytes,
		})
	}
	for _, age := range stats.Ages {
		dashboard.Ages = append(dashboard.Ages, ui.CacheDashboardAge{
			Bucket:          age.Bucket,
			MessageCount:    age.MessageCount,
			BodyCount:       age.BodyCount,
			AttachmentCount: age.AttachmentCount,
			TotalBytes:      age.TotalBytes,
		})
	}
	for _, row := range stats.Rows {
		dashboard.Rows = append(dashboard.Rows, ui.CacheDashboardRow{Table: row.Table, Rows: row.Rows})
	}
	return dashboard
}

func buildCachePolicyStore(paths config.Paths, cfg config.Config) ui.CachePolicyStore {
	return ui.CachePolicyStoreFunc{
		LoadFn: func() (ui.CachePolicyTable, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CachePolicyTable{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CachePolicyTable{}, err
			}
			return loadCachePolicyTable(ctx, db, cfg)
		},
		SaveFn: func(row ui.CachePolicyRow) (ui.CachePolicyRow, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CachePolicyRow{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CachePolicyRow{}, err
			}
			if _, err := cache.NewSyncPolicyEditor(db).Save(ctx, cachePolicyFromRow(row)); err != nil {
				return ui.CachePolicyRow{}, err
			}
			effective, err := gandtsync.NewPolicyEvaluator(db, cfg).EffectiveForLabel(ctx, row.AccountID, row.AccountEmail, row.LabelID)
			if err != nil {
				return ui.CachePolicyRow{}, err
			}
			return cachePolicyRowFromLabelPolicy(row, effective, true), nil
		},
		ResetFn: func(row ui.CachePolicyRow) (ui.CachePolicyRow, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CachePolicyRow{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CachePolicyRow{}, err
			}
			if _, err := cache.NewSyncPolicyEditor(db).ResetToDefault(ctx, row.AccountID, row.LabelID); err != nil {
				return ui.CachePolicyRow{}, err
			}
			effective, err := gandtsync.NewPolicyEvaluator(db, cfg).EffectiveForLabel(ctx, row.AccountID, row.AccountEmail, row.LabelID)
			if err != nil {
				return ui.CachePolicyRow{}, err
			}
			return cachePolicyRowFromLabelPolicy(row, effective, false), nil
		},
	}
}

func loadCachePolicyTable(ctx context.Context, db *sqlx.DB, cfg config.Config) (ui.CachePolicyTable, error) {
	accounts, err := cache.NewAccountRepository(db).List(ctx)
	if err != nil {
		return ui.CachePolicyTable{}, err
	}
	labels := cache.NewLabelRepository(db)
	policies := cache.NewSyncPolicyRepository(db)
	evaluator := gandtsync.NewPolicyEvaluator(db, cfg)

	table := ui.CachePolicyTable{}
	for _, account := range accounts {
		accountLabels, err := labels.List(ctx, account.ID)
		if err != nil {
			return ui.CachePolicyTable{}, err
		}
		for _, label := range accountLabels {
			effective, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, label.ID)
			if err != nil {
				return ui.CachePolicyTable{}, err
			}
			_, explicitErr := policies.Get(ctx, account.ID, label.ID)
			explicit := explicitErr == nil
			if explicitErr != nil && !errors.Is(explicitErr, cache.ErrSyncPolicyNotFound) {
				return ui.CachePolicyTable{}, explicitErr
			}
			table.Rows = append(table.Rows, ui.CachePolicyRow{
				AccountID:       account.ID,
				AccountEmail:    account.Email,
				LabelID:         label.ID,
				LabelName:       label.Name,
				Explicit:        explicit,
				Depth:           string(effective.Depth),
				RetentionDays:   cloneMainInt(effective.RetentionDays),
				AttachmentRule:  string(effective.AttachmentRule),
				AttachmentMaxMB: cloneMainInt(effective.AttachmentMaxMB),
			})
		}
	}
	return table, nil
}

func cachePolicyFromRow(row ui.CachePolicyRow) cache.SyncPolicy {
	return cache.SyncPolicy{
		AccountID:       row.AccountID,
		LabelID:         row.LabelID,
		Include:         row.Depth != "none",
		Depth:           row.Depth,
		RetentionDays:   cloneMainInt(row.RetentionDays),
		AttachmentRule:  row.AttachmentRule,
		AttachmentMaxMB: cloneMainInt(row.AttachmentMaxMB),
	}
}

func cachePolicyRowFromLabelPolicy(row ui.CachePolicyRow, policy gandtsync.LabelPolicy, explicit bool) ui.CachePolicyRow {
	row.Explicit = explicit
	if policy.Depth != "" {
		row.Depth = string(policy.Depth)
	}
	if policy.AttachmentRule != "" {
		row.AttachmentRule = string(policy.AttachmentRule)
	}
	row.RetentionDays = cloneMainInt(policy.RetentionDays)
	row.AttachmentMaxMB = cloneMainInt(policy.AttachmentMaxMB)
	return row
}

func cloneMainInt(value *int) *int {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func buildCacheExclusionStore(paths config.Paths) ui.CacheExclusionStore {
	return ui.CacheExclusionStoreFunc{
		PreviewFn: func(request ui.CacheExclusionRequest) (ui.CacheExclusionPreview, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CacheExclusionPreview{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CacheExclusionPreview{}, err
			}
			account, err := cacheExclusionAccount(ctx, db, request.Account)
			if err != nil {
				return ui.CacheExclusionPreview{}, err
			}
			plan, err := cache.NewCacheExclusionService(db).PreviewPurge(ctx, cache.CacheExclusion{
				AccountID:  account.ID,
				MatchType:  request.MatchType,
				MatchValue: request.MatchValue,
			})
			if err != nil {
				return ui.CacheExclusionPreview{}, err
			}
			return uiCacheExclusionPreview(request, plan), nil
		},
		ConfirmFn: func(request ui.CacheExclusionRequest) (ui.CacheExclusionResult, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CacheExclusionResult{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CacheExclusionResult{}, err
			}
			account, err := cacheExclusionAccount(ctx, db, request.Account)
			if err != nil {
				return ui.CacheExclusionResult{}, err
			}
			result, err := cache.NewCacheExclusionService(db).ConfirmPurge(ctx, cache.CacheExclusion{
				AccountID:  account.ID,
				MatchType:  request.MatchType,
				MatchValue: request.MatchValue,
			})
			if err != nil {
				return ui.CacheExclusionResult{}, err
			}
			return ui.CacheExclusionResult{
				Preview:                uiCacheExclusionPreview(request, result.Plan),
				DeletedMessages:        result.DeletedMessages,
				DeletedAttachmentFiles: result.DeletedAttachmentFiles,
				AttachmentDeleteErrors: result.AttachmentDeleteErrors,
			}, nil
		},
	}
}

func cacheExclusionAccount(ctx context.Context, db *sqlx.DB, accountName string) (cache.Account, error) {
	accounts, err := cache.NewAccountRepository(db).List(ctx)
	if err != nil {
		return cache.Account{}, err
	}
	if len(accounts) == 0 {
		return cache.Account{}, fmt.Errorf("no accounts configured")
	}
	for _, account := range accounts {
		if strings.EqualFold(account.Email, accountName) || account.ID == accountName {
			return account, nil
		}
	}
	if accountName != "" {
		return cache.Account{}, fmt.Errorf("account %q not found", accountName)
	}
	return accounts[0], nil
}

func uiCacheExclusionPreview(request ui.CacheExclusionRequest, plan cache.CacheExclusionPurgePlan) ui.CacheExclusionPreview {
	return ui.CacheExclusionPreview{
		Request:         request,
		MessageCount:    plan.MessageCount,
		BodyCount:       plan.BodyCount,
		AttachmentCount: plan.AttachmentCount,
		EstimatedBytes:  plan.EstimatedBytes,
	}
}

func buildCachePurgeStore(paths config.Paths) ui.CachePurgeStore {
	return ui.CachePurgeStoreFunc{
		PlanFn: func(request ui.CachePurgeRequest) (ui.CachePurgePreview, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CachePurgePreview{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CachePurgePreview{}, err
			}
			filter, err := cachePurgeFilter(ctx, db, request)
			if err != nil {
				return ui.CachePurgePreview{}, err
			}
			plan, err := cache.NewCachePurgeService(db).Plan(ctx, filter, time.Now().UTC())
			if err != nil {
				return ui.CachePurgePreview{}, err
			}
			return uiCachePurgePreview(request, plan), nil
		},
		ExecuteFn: func(request ui.CachePurgeRequest) (ui.CachePurgeResult, error) {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return ui.CachePurgeResult{}, err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return ui.CachePurgeResult{}, err
			}
			filter, err := cachePurgeFilter(ctx, db, request)
			if err != nil {
				return ui.CachePurgeResult{}, err
			}
			result, err := cache.NewCachePurgeService(db).Execute(ctx, filter, time.Now().UTC())
			if err != nil {
				return ui.CachePurgeResult{}, err
			}
			return ui.CachePurgeResult{
				Preview:                uiCachePurgePreview(request, result.Plan),
				DeletedMessages:        result.DeletedMessages,
				DeletedAttachmentFiles: result.DeletedAttachmentFiles,
				AttachmentDeleteErrors: result.AttachmentDeleteErrors,
			}, nil
		},
		CompactFn: func() error {
			ctx := context.Background()
			db, err := cache.Open(ctx, paths)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := cache.Migrate(ctx, db); err != nil {
				return err
			}
			return cache.NewCachePurgeService(db).Compact(ctx)
		},
	}
}

func cachePurgeFilter(ctx context.Context, db *sqlx.DB, request ui.CachePurgeRequest) (cache.CachePurgeFilter, error) {
	accountID := ""
	if request.Account != "" {
		account, err := cacheExclusionAccount(ctx, db, request.Account)
		if err != nil {
			return cache.CachePurgeFilter{}, err
		}
		accountID = account.ID
	}
	return cache.CachePurgeFilter{
		AccountID:     accountID,
		LabelID:       request.LabelID,
		OlderThanDays: request.OlderThanDays,
		From:          request.From,
		DryRun:        request.DryRun,
	}, nil
}

func uiCachePurgePreview(request ui.CachePurgeRequest, plan cache.CachePurgePlan) ui.CachePurgePreview {
	return ui.CachePurgePreview{
		Request:         request,
		MessageCount:    plan.MessageCount,
		BodyCount:       plan.BodyCount,
		AttachmentCount: plan.AttachmentCount,
		EstimatedBytes:  plan.EstimatedBytes,
	}
}

func buildCacheWipeStore(paths config.Paths) ui.CacheWipeStore {
	return ui.CacheWipeStoreFunc(func() (ui.CacheWipeResult, error) {
		result, err := cache.Wipe(context.Background(), paths)
		if err != nil {
			return ui.CacheWipeResult{}, err
		}
		return ui.CacheWipeResult{
			DatabaseFilesRemoved:   result.DatabaseFilesRemoved,
			AttachmentFilesRemoved: result.AttachmentFilesRemoved,
			AttachmentDeleteErrors: result.AttachmentDeleteErrors,
		}, nil
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
		var backfill gandtsync.BackfillResult
		if request.LabelID == "" {
			backfill, err = backfiller.Backfill(ctx, account)
		} else {
			backfill, err = backfiller.BackfillLabel(ctx, account, request.LabelID)
		}
		if err != nil {
			return gandtsync.AccountSyncResult{}, err
		}
		return gandtsync.AccountSyncResult{
			Backfill: backfill,
			Status:   fmt.Sprintf("refreshed %s", firstNonEmptyString(request.LabelName, request.LabelID, "label")),
		}, nil
	}
	return gandtsync.NewDeltaSynchronizer(db, cfg, gmailClient, gandtsync.WithLogger(charmSyncLogger{})).Sync(ctx, account)
}

func buildTriageActor(paths config.Paths) ui.TriageActor {
	return ui.TriageActorFunc(func(request ui.TriageActionRequest) (ui.TriageActionResult, error) {
		ctx := context.Background()
		db, err := cache.Open(ctx, paths)
		if err != nil {
			return ui.TriageActionResult{}, err
		}
		defer db.Close()
		if err := cache.Migrate(ctx, db); err != nil {
			return ui.TriageActionResult{}, err
		}

		accounts, err := cache.NewAccountRepository(db).List(ctx)
		if err != nil {
			return ui.TriageActionResult{}, err
		}
		if len(accounts) == 0 {
			return ui.TriageActionResult{}, fmt.Errorf("no accounts configured")
		}
		account := accounts[0]
		for _, candidate := range accounts {
			if candidate.Email == request.Account {
				account = candidate
				break
			}
		}
		actionStarted := time.Now()
		charmlog.Info("action_attempt",
			"account_id", account.ID,
			"email", account.Email,
			"kind", request.Kind,
			"message_id", request.MessageID,
			"thread_id", request.ThreadID,
		)

		gmailClient, err := gmailClientForAccount(ctx, account.ID)
		if err != nil {
			charmlog.Error("action_failure", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds(), "error", err)
			return ui.TriageActionResult{}, err
		}
		if request.CreateLabel {
			label, err := gmailClient.CreateLabel(ctx, gandtgmail.LabelCreateRequest{Name: request.LabelName})
			if err != nil {
				charmlog.Error("action_failure", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds(), "error", err)
				return ui.TriageActionResult{}, err
			}
			request.LabelID = label.ID
			if err := cache.NewLabelRepository(db).Upsert(ctx, cache.Label{
				AccountID: account.ID,
				ID:        label.ID,
				Name:      label.Name,
				Type:      firstNonEmptyString(label.Type, "user"),
				Unread:    label.Unread,
				Total:     label.Total,
				ColorBG:   label.ColorBG,
				ColorFG:   label.ColorFG,
			}); err != nil {
				charmlog.Error("action_failure", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds(), "error", err)
				return ui.TriageActionResult{}, err
			}
		}

		repo := cache.NewOptimisticActionRepository(db)
		snapshot, err := repo.Apply(ctx, cacheActionForRequest(account.ID, request))
		if err != nil {
			charmlog.Error("action_failure", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds(), "error", err)
			return ui.TriageActionResult{}, err
		}
		if err := dispatchGmailAction(ctx, gmailClient, request); err != nil {
			_ = repo.Revert(ctx, snapshot)
			charmlog.Error("action_failure", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds(), "error", err)
			return ui.TriageActionResult{}, err
		}
		charmlog.Info("action_success", "account_id", account.ID, "kind", request.Kind, "duration_ms", time.Since(actionStarted).Milliseconds())
		return ui.TriageActionResult{
			Summary:   triageActionSummary(request),
			LabelID:   request.LabelID,
			LabelName: request.LabelName,
		}, nil
	})
}

type charmSyncLogger struct{}

func (charmSyncLogger) LogSyncEvent(event string, fields map[string]any) {
	args := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		args = append(args, key, value)
	}
	charmlog.Info(event, args...)
}

func cacheActionForRequest(accountID string, request ui.TriageActionRequest) cache.OptimisticAction {
	action := cache.OptimisticAction{
		AccountID: accountID,
		MessageID: request.MessageID,
		LabelID:   request.LabelID,
		Add:       request.Add,
	}
	switch request.Kind {
	case ui.TriageArchive:
		action.Kind = cache.OptimisticArchive
	case ui.TriageTrash:
		action.Kind = cache.OptimisticTrash
	case ui.TriageUntrash:
		action.Kind = cache.OptimisticUntrash
	case ui.TriageSpam:
		action.Kind = cache.OptimisticSpam
	case ui.TriageUnspam:
		action.Kind = cache.OptimisticUnspam
	case ui.TriageStar:
		action.Kind = cache.OptimisticToggleStar
	case ui.TriageUnread:
		action.Kind = cache.OptimisticToggleUnread
	case ui.TriageLabelAdd:
		action.Kind = cache.OptimisticLabelAdd
	case ui.TriageLabelRemove:
		action.Kind = cache.OptimisticLabelRemove
	case ui.TriageMute:
		action.Kind = cache.OptimisticMute
	}
	return action
}

func dispatchGmailAction(ctx context.Context, client *gandtgmail.Client, request ui.TriageActionRequest) error {
	switch request.Kind {
	case ui.TriageArchive:
		return client.BatchModifyMessages(ctx, gandtgmail.MessageModifyRequest{IDs: []string{request.MessageID}, RemoveLabelIDs: []string{"INBOX"}})
	case ui.TriageTrash:
		return client.TrashMessage(ctx, request.MessageID)
	case ui.TriageUntrash:
		return client.UntrashMessage(ctx, request.MessageID)
	case ui.TriageSpam:
		return client.BatchModifyMessages(ctx, gandtgmail.MessageModifyRequest{IDs: []string{request.MessageID}, AddLabelIDs: []string{"SPAM"}, RemoveLabelIDs: []string{"INBOX"}})
	case ui.TriageUnspam:
		return client.BatchModifyMessages(ctx, gandtgmail.MessageModifyRequest{IDs: []string{request.MessageID}, AddLabelIDs: []string{"INBOX"}, RemoveLabelIDs: []string{"SPAM"}})
	case ui.TriageStar:
		return client.BatchModifyMessages(ctx, labelToggleRequest(request.MessageID, "STARRED", request.Add))
	case ui.TriageUnread:
		return client.BatchModifyMessages(ctx, labelToggleRequest(request.MessageID, "UNREAD", request.Add))
	case ui.TriageLabelAdd:
		return client.BatchModifyMessages(ctx, gandtgmail.MessageModifyRequest{IDs: []string{request.MessageID}, AddLabelIDs: []string{request.LabelID}})
	case ui.TriageLabelRemove:
		return client.BatchModifyMessages(ctx, gandtgmail.MessageModifyRequest{IDs: []string{request.MessageID}, RemoveLabelIDs: []string{request.LabelID}})
	case ui.TriageMute:
		return client.ModifyThread(ctx, gandtgmail.ThreadModifyRequest{ThreadID: request.ThreadID, AddLabelIDs: []string{"MUTED"}})
	default:
		return fmt.Errorf("unsupported action %q", request.Kind)
	}
}

func labelToggleRequest(messageID string, labelID string, add bool) gandtgmail.MessageModifyRequest {
	request := gandtgmail.MessageModifyRequest{IDs: []string{messageID}}
	if add {
		request.AddLabelIDs = []string{labelID}
	} else {
		request.RemoveLabelIDs = []string{labelID}
	}
	return request
}

func triageActionSummary(request ui.TriageActionRequest) string {
	switch request.Kind {
	case ui.TriageArchive:
		return "archived"
	case ui.TriageTrash:
		return "trashed"
	case ui.TriageUntrash:
		return "restored from trash"
	case ui.TriageSpam:
		return "marked spam"
	case ui.TriageUnspam:
		return "restored from spam"
	case ui.TriageStar:
		if request.Add {
			return "starred"
		}
		return "unstarred"
	case ui.TriageUnread:
		if request.Add {
			return "marked unread"
		}
		return "marked read"
	case ui.TriageLabelAdd:
		return "label added"
	case ui.TriageLabelRemove:
		return "label removed"
	case ui.TriageMute:
		return "muted"
	default:
		return "action complete"
	}
}

func loadInitialMailbox(paths config.Paths, cfg config.Config) (ui.Mailbox, bool) {
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
	if _, err := gandtsync.NewRetentionSweeper(db, cfg).Sweep(ctx, account, time.Now().UTC()); err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	labels, err := cache.NewLabelRepository(db).List(ctx, account.ID)
	if err != nil {
		return ui.AuthFailureMailbox(err.Error()), true
	}
	labelsForUI := uiLabels(ctx, db, cfg, account, labels)
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

func uiLabels(ctx context.Context, db *sqlx.DB, cfg config.Config, account cache.Account, labels []cache.Label) []ui.Label {
	evaluator := gandtsync.NewPolicyEvaluator(db, cfg)
	out := make([]ui.Label, 0, len(labels))
	for _, label := range labels {
		depth := ""
		if policy, err := evaluator.EffectiveForLabel(ctx, account.ID, account.Email, label.ID); err == nil {
			depth = string(policy.Depth)
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
