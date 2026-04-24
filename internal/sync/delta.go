package sync

import (
	"context"
	"errors"
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
}

type DeltaSyncResult struct {
	HistoryID       string
	MessagesAdded   int
	MessagesDeleted int
	LabelsAdded     int
	LabelsRemoved   int
	BodyFetches     int
}

func NewDeltaSynchronizer(db *sqlx.DB, cfg config.Config, client gmail.MessageReader) DeltaSynchronizer {
	return DeltaSynchronizer{
		accounts:  cache.NewAccountRepository(db),
		messages:  cache.NewMessageRepository(db),
		msgLabels: cache.NewMessageLabelRepository(db),
		backfill:  NewBackfiller(db, cfg, client),
		gmail:     client,
	}
}

func (s DeltaSynchronizer) DeltaSync(ctx context.Context, account cache.Account) (DeltaSyncResult, error) {
	if account.HistoryID == "" {
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
			return DeltaSyncResult{}, err
		}
		if page.HistoryID != "" {
			result.HistoryID = page.HistoryID
		}
		for _, record := range page.Records {
			if err := s.applyHistoryRecord(ctx, account, record, &result); err != nil {
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
			return DeltaSyncResult{}, err
		}
	}
	return result, nil
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
