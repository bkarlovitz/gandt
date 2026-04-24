package sync

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/jmoiron/sqlx"
)

const historyPageSize = 500

type DeltaSynchronizer struct {
	accounts  cache.AccountRepository
	messages  cache.MessageRepository
	msgLabels cache.MessageLabelRepository
	backfill  Backfiller
	gmail     gmail.MessageReader
	logger    Logger
}

type DeltaSyncResult struct {
	HistoryID       string
	MessagesAdded   int
	MessagesDeleted int
	LabelsAdded     int
	LabelsRemoved   int
	BodyFetches     int
}

type AccountSyncResult struct {
	Delta    DeltaSyncResult
	Backfill BackfillResult
	Bodies   BodyFetchResult
	Fallback bool
	Status   string
}

func NewDeltaSynchronizer(db *sqlx.DB, cfg config.Config, client gmail.MessageReader, opts ...DeltaOption) DeltaSynchronizer {
	synchronizer := DeltaSynchronizer{
		accounts:  cache.NewAccountRepository(db),
		messages:  cache.NewMessageRepository(db),
		msgLabels: cache.NewMessageLabelRepository(db),
		backfill:  NewBackfiller(db, cfg, client),
		gmail:     client,
		logger:    noopLogger{},
	}
	for _, opt := range opts {
		opt(&synchronizer)
	}
	return synchronizer
}

func (s DeltaSynchronizer) Sync(ctx context.Context, account cache.Account) (AccountSyncResult, error) {
	started := time.Now()
	s.log("sync_start", account, map[string]any{"mode": "delta"})
	delta, err := s.DeltaSync(ctx, account)
	if err == nil {
		s.log("sync_success", account, map[string]any{"mode": "delta", "duration_ms": durationMillis(started)})
		return AccountSyncResult{Delta: delta, Status: "sync complete"}, nil
	}
	if !isExpiredHistoryError(err) {
		s.log("sync_failure", account, map[string]any{"mode": "delta", "duration_ms": durationMillis(started), "error": err.Error()})
		return AccountSyncResult{}, err
	}
	s.log("sync_history_expired", account, map[string]any{"status": "fallback"})

	backfill, err := s.backfill.Backfill(ctx, account)
	if err != nil {
		s.log("sync_failure", account, map[string]any{"mode": "fallback", "duration_ms": durationMillis(started), "error": err.Error()})
		return AccountSyncResult{}, err
	}
	bodyQueue, err := s.missingBodyQueue(ctx, account.ID, backfill.BodyQueue)
	if err != nil {
		return AccountSyncResult{}, err
	}
	bodies, err := s.backfill.FetchBodies(ctx, account, bodyQueue)
	if err != nil {
		s.log("sync_failure", account, map[string]any{"mode": "fallback", "duration_ms": durationMillis(started), "error": err.Error()})
		return AccountSyncResult{}, err
	}
	if historyID := latestHistoryID(backfill.Messages); historyID != "" {
		if err := s.accounts.UpdateSyncMetadata(ctx, account.ID, historyID, time.Now().UTC()); err != nil {
			s.log("sync_failure", account, map[string]any{"mode": "fallback", "duration_ms": durationMillis(started), "error": err.Error()})
			return AccountSyncResult{}, err
		}
	}
	s.log("sync_success", account, map[string]any{
		"mode":         "fallback",
		"duration_ms":  durationMillis(started),
		"messages":     len(backfill.Messages),
		"body_fetches": bodies.Fetched,
	})

	return AccountSyncResult{
		Backfill: backfill,
		Bodies:   bodies,
		Fallback: true,
		Status:   "Gmail history expired; refreshing mailbox",
	}, nil
}

func (s DeltaSynchronizer) DeltaSync(ctx context.Context, account cache.Account) (DeltaSyncResult, error) {
	started := time.Now()
	s.log("delta_sync_start", account, nil)
	if account.HistoryID == "" {
		s.log("delta_sync_failure", account, map[string]any{"duration_ms": durationMillis(started), "error": "account history id is required"})
		return DeltaSyncResult{}, errors.New("account history id is required for delta sync")
	}

	result := DeltaSyncResult{}
	pageToken := ""
	for {
		page, err := s.gmail.ListHistory(ctx, gmail.ListHistoryOptions{
			StartHistoryID: account.HistoryID,
			PageToken:      pageToken,
			MaxResults:     historyPageSize,
		})
		if err != nil {
			s.log("delta_sync_failure", account, map[string]any{"duration_ms": durationMillis(started), "error": err.Error()})
			return DeltaSyncResult{}, err
		}
		if page.HistoryID != "" {
			result.HistoryID = page.HistoryID
		}
		for _, record := range page.Records {
			if err := s.applyHistoryRecord(ctx, account, record, &result); err != nil {
				s.log("delta_sync_failure", account, map[string]any{"duration_ms": durationMillis(started), "error": err.Error()})
				return DeltaSyncResult{}, err
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	if result.HistoryID != "" {
		if err := s.accounts.UpdateSyncMetadata(ctx, account.ID, result.HistoryID, time.Now().UTC()); err != nil {
			s.log("delta_sync_failure", account, map[string]any{"duration_ms": durationMillis(started), "error": err.Error()})
			return DeltaSyncResult{}, err
		}
	}
	s.log("delta_sync_success", account, map[string]any{
		"duration_ms":      durationMillis(started),
		"messages_added":   result.MessagesAdded,
		"messages_deleted": result.MessagesDeleted,
		"labels_added":     result.LabelsAdded,
		"labels_removed":   result.LabelsRemoved,
	})
	return result, nil
}

func (s DeltaSynchronizer) log(event string, account cache.Account, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["account_id"] = account.ID
	if account.Email != "" {
		fields["email"] = account.Email
	}
	s.logger.LogSyncEvent(event, fields)
}

func durationMillis(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func (s DeltaSynchronizer) applyHistoryRecord(ctx context.Context, account cache.Account, record gmail.HistoryRecord, result *DeltaSyncResult) error {
	for _, change := range record.MessagesAdded {
		if err := s.fetchAndPersistChangedMessage(ctx, account, change.Message); err != nil {
			return err
		}
		result.MessagesAdded++
	}
	for _, change := range record.MessagesDeleted {
		if change.Message.ID == "" {
			continue
		}
		if err := s.messages.Delete(ctx, account.ID, change.Message.ID); err != nil && !errors.Is(err, cache.ErrMessageNotFound) {
			return err
		}
		result.MessagesDeleted++
	}
	for _, change := range record.LabelsAdded {
		if change.Message.ID == "" {
			continue
		}
		if err := s.ensureChangedMessageCached(ctx, account, change.Message); err != nil {
			return err
		}
		for _, labelID := range change.LabelIDs {
			if err := s.msgLabels.Upsert(ctx, cache.MessageLabel{AccountID: account.ID, MessageID: change.Message.ID, LabelID: labelID}); err != nil {
				return err
			}
			result.LabelsAdded++
		}
	}
	for _, change := range record.LabelsRemoved {
		if change.Message.ID == "" {
			continue
		}
		for _, labelID := range change.LabelIDs {
			if err := s.msgLabels.Delete(ctx, account.ID, change.Message.ID, labelID); err != nil && !errors.Is(err, cache.ErrMessageLabelAbsent) {
				return err
			}
			result.LabelsRemoved++
		}
	}
	return nil
}

func (s DeltaSynchronizer) missingBodyQueue(ctx context.Context, accountID string, queue []BodyFetchRequest) ([]BodyFetchRequest, error) {
	out := make([]BodyFetchRequest, 0, len(queue))
	for _, request := range queue {
		message, err := s.messages.Get(ctx, accountID, request.MessageID)
		if errors.Is(err, cache.ErrMessageNotFound) {
			out = append(out, request)
			continue
		}
		if err != nil {
			return nil, err
		}
		if message.BodyPlain == nil && message.BodyHTML == nil && message.CachedAt == nil {
			out = append(out, request)
		}
	}
	return out, nil
}

func isExpiredHistoryError(err error) bool {
	return errors.Is(err, gmail.ErrHistoryGone) || errors.Is(err, gmail.ErrNotFound)
}

func latestHistoryID(messages []gmail.Message) string {
	var latest uint64
	out := ""
	for _, message := range messages {
		if message.HistoryID == "" {
			continue
		}
		parsed, err := strconv.ParseUint(message.HistoryID, 10, 64)
		if err != nil {
			out = message.HistoryID
			continue
		}
		if parsed >= latest {
			latest = parsed
			out = message.HistoryID
		}
	}
	return out
}

func (s DeltaSynchronizer) ensureChangedMessageCached(ctx context.Context, account cache.Account, ref gmail.MessageRef) error {
	if ref.ID == "" {
		return nil
	}
	if _, err := s.messages.Get(ctx, account.ID, ref.ID); err == nil {
		return nil
	} else if !errors.Is(err, cache.ErrMessageNotFound) {
		return err
	}
	return s.fetchAndPersistChangedMessage(ctx, account, ref)
}

func (s DeltaSynchronizer) fetchAndPersistChangedMessage(ctx context.Context, account cache.Account, ref gmail.MessageRef) error {
	if ref.ID == "" {
		return nil
	}
	message, err := s.gmail.GetMessageMetadata(ctx, ref.ID, MetadataHeaders...)
	if err != nil {
		return err
	}
	message.ThreadID = firstNonEmpty(message.ThreadID, ref.ThreadID)
	labelIDs := mergeLabels(nil, message.LabelIDs)
	decision, err := s.backfill.evaluator.Evaluate(ctx, MessageContext{
		AccountID:    account.ID,
		AccountEmail: account.Email,
		From:         headerValue(message.Headers, "From"),
		LabelIDs:     labelIDs,
	})
	if err != nil {
		return err
	}
	if decision.Excluded || !decision.Persist {
		return nil
	}
	if decision.Depth == config.CacheDepthBody || decision.Depth == config.CacheDepthFull {
		full, err := s.gmail.GetMessageFull(ctx, ref.ID)
		if err != nil {
			return err
		}
		full.ThreadID = firstNonEmpty(full.ThreadID, message.ThreadID, ref.ThreadID)
		_, err = s.backfill.PersistFullMessage(ctx, account, full)
		return err
	}
	return s.backfill.persistMetadata(ctx, account.ID, message, labelIDs)
}
