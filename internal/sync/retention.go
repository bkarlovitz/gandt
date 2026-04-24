package sync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/jmoiron/sqlx"
)

type RetentionSweeper struct {
	db        *sqlx.DB
	evaluator PolicyEvaluator
	labels    cache.MessageLabelRepository
	purge     cache.CachePurgeService
}

type RetentionSweepResult struct {
	Checked          int
	Purged           int
	ExcludedPurged   int
	AttachmentErrors []string
}

type retentionMessageRow struct {
	AccountID    string       `db:"account_id"`
	MessageID    string       `db:"message_id"`
	FromAddr     string       `db:"from_addr"`
	InternalDate sql.NullTime `db:"internal_date"`
	Date         sql.NullTime `db:"date"`
	CachedAt     sql.NullTime `db:"cached_at"`
}

func NewRetentionSweeper(db *sqlx.DB, cfg config.Config) RetentionSweeper {
	return RetentionSweeper{
		db:        db,
		evaluator: NewPolicyEvaluator(db, cfg),
		labels:    cache.NewMessageLabelRepository(db),
		purge:     cache.NewCachePurgeService(db),
	}
}

func (s RetentionSweeper) Sweep(ctx context.Context, account cache.Account, now time.Time) (RetentionSweepResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rows := []retentionMessageRow{}
	if err := s.db.SelectContext(ctx, &rows, `
SELECT account_id, id AS message_id, COALESCE(from_addr, '') AS from_addr, internal_date, date, cached_at
FROM messages
WHERE account_id = ?
ORDER BY id
`, account.ID); err != nil {
		return RetentionSweepResult{}, fmt.Errorf("list retention candidates: %w", err)
	}

	result := RetentionSweepResult{Checked: len(rows)}
	keys := []cache.MessageKey{}
	for _, row := range rows {
		labelRows, err := s.labels.ListForMessage(ctx, row.AccountID, row.MessageID)
		if err != nil {
			return RetentionSweepResult{}, err
		}
		labelIDs := retentionLabelIDs(labelRows)
		decision, err := s.evaluator.Evaluate(ctx, MessageContext{
			AccountID:    account.ID,
			AccountEmail: account.Email,
			From:         row.FromAddr,
			LabelIDs:     labelIDs,
		})
		if err != nil {
			return RetentionSweepResult{}, err
		}
		if decision.Excluded {
			keys = append(keys, cache.MessageKey{AccountID: row.AccountID, MessageID: row.MessageID})
			result.ExcludedPurged++
			continue
		}
		if retentionExpired(decision.Policies, retentionMessageTime(row), now) {
			keys = append(keys, cache.MessageKey{AccountID: row.AccountID, MessageID: row.MessageID})
		}
	}
	if len(keys) == 0 {
		return result, nil
	}
	purged, err := s.purge.ExecutePlan(ctx, cache.CachePurgePlan{MessageKeys: keys})
	if err != nil {
		return RetentionSweepResult{}, err
	}
	result.Purged = purged.DeletedMessages
	result.AttachmentErrors = purged.AttachmentDeleteErrors
	return result, nil
}

func retentionExpired(policies []LabelPolicy, messageTime time.Time, now time.Time) bool {
	if messageTime.IsZero() || len(policies) == 0 {
		return false
	}
	for _, policy := range policies {
		if policy.RetentionDays == nil || *policy.RetentionDays <= 0 {
			return false
		}
		if !messageTime.Before(now.AddDate(0, 0, -*policy.RetentionDays)) {
			return false
		}
	}
	return true
}

func retentionMessageTime(row retentionMessageRow) time.Time {
	for _, value := range []sql.NullTime{row.InternalDate, row.Date, row.CachedAt} {
		if value.Valid {
			return value.Time
		}
	}
	return time.Time{}
}

func retentionLabelIDs(labels []cache.MessageLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, label.LabelID)
	}
	return out
}

type RetentionSchedule struct {
	lastRun map[string]time.Time
}

func NewRetentionSchedule() *RetentionSchedule {
	return &RetentionSchedule{lastRun: map[string]time.Time{}}
}

func (s *RetentionSchedule) ShouldRun(accountID string, now time.Time) bool {
	if s == nil {
		return true
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	last, ok := s.lastRun[accountID]
	if !ok || now.Sub(last) >= 24*time.Hour {
		s.lastRun[accountID] = now
		return true
	}
	return false
}
