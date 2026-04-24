package cache

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type CachePurgeService struct {
	db *sqlx.DB
}

type CachePurgeFilter struct {
	AccountID     string
	LabelID       string
	OlderThanDays int
	From          string
	DryRun        bool
}

type CachePurgePlan struct {
	Filter          CachePurgeFilter
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	EstimatedBytes  int64
	MessageKeys     []MessageKey
}

type CachePurgeResult struct {
	Plan                   CachePurgePlan
	DeletedMessages        int
	DeletedAttachmentFiles int
	AttachmentDeleteErrors []string
	Checkpointed           bool
}

type purgeCandidate struct {
	AccountID string `db:"account_id"`
	MessageID string `db:"message_id"`
	FromAddr  string `db:"from_addr"`
}

func NewCachePurgeService(db *sqlx.DB) CachePurgeService {
	return CachePurgeService{db: db}
}

func (s CachePurgeService) Plan(ctx context.Context, filter CachePurgeFilter, now time.Time) (CachePurgePlan, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	filter.AccountID = strings.TrimSpace(filter.AccountID)
	filter.LabelID = strings.TrimSpace(filter.LabelID)
	filter.From = senderAddressForCache(filter.From)

	candidates, err := s.purgeCandidates(ctx, filter, now)
	if err != nil {
		return CachePurgePlan{}, err
	}
	keys := make([]MessageKey, 0, len(candidates))
	for _, candidate := range candidates {
		if filter.From != "" && senderAddressForCache(candidate.FromAddr) != filter.From {
			continue
		}
		keys = append(keys, MessageKey{AccountID: candidate.AccountID, MessageID: candidate.MessageID})
	}

	plan := CachePurgePlan{Filter: filter, MessageKeys: keys}
	if len(keys) == 0 {
		return plan, nil
	}
	if err := s.populatePurgePlanCounts(ctx, &plan); err != nil {
		return CachePurgePlan{}, err
	}
	return plan, nil
}

func (s CachePurgeService) Execute(ctx context.Context, filter CachePurgeFilter, now time.Time) (CachePurgeResult, error) {
	plan, err := s.Plan(ctx, filter, now)
	if err != nil {
		return CachePurgeResult{}, err
	}
	return s.ExecutePlan(ctx, plan)
}

func (s CachePurgeService) ExecutePlan(ctx context.Context, plan CachePurgePlan) (CachePurgeResult, error) {
	if len(plan.MessageKeys) > 0 && plan.MessageCount == 0 {
		if err := s.populatePurgePlanCounts(ctx, &plan); err != nil {
			return CachePurgeResult{}, err
		}
	}
	result := CachePurgeResult{Plan: plan}
	if len(plan.MessageKeys) == 0 {
		return result, nil
	}

	paths, err := s.attachmentPaths(ctx, plan.MessageKeys)
	if err != nil {
		return CachePurgeResult{}, err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			result.AttachmentDeleteErrors = append(result.AttachmentDeleteErrors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		result.DeletedAttachmentFiles++
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return CachePurgeResult{}, fmt.Errorf("begin cache purge: %w", err)
	}
	defer tx.Rollback()
	for accountID, ids := range keysByAccount(plan.MessageKeys) {
		query, args, err := sqlx.In("DELETE FROM messages WHERE account_id = ? AND id IN (?)", accountID, ids)
		if err != nil {
			return CachePurgeResult{}, err
		}
		deleteResult, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return CachePurgeResult{}, fmt.Errorf("delete cache purge messages: %w", err)
		}
		affected, err := deleteResult.RowsAffected()
		if err != nil {
			return CachePurgeResult{}, fmt.Errorf("read cache purge delete count: %w", err)
		}
		result.DeletedMessages += int(affected)
	}
	if err := tx.Commit(); err != nil {
		return CachePurgeResult{}, fmt.Errorf("commit cache purge: %w", err)
	}
	if err := s.Checkpoint(ctx); err != nil {
		return CachePurgeResult{}, err
	}
	result.Checkpointed = true
	return result, nil
}

func (s CachePurgeService) Checkpoint(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint cache WAL: %w", err)
	}
	return nil
}

func (s CachePurgeService) Compact(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("compact cache database: %w", err)
	}
	return nil
}

func (s CachePurgeService) purgeCandidates(ctx context.Context, filter CachePurgeFilter, now time.Time) ([]purgeCandidate, error) {
	query := `
SELECT m.account_id AS account_id,
       m.id AS message_id,
       COALESCE(m.from_addr, '') AS from_addr
FROM messages m
WHERE 1 = 1
`
	args := []any{}
	if filter.AccountID != "" {
		query += " AND m.account_id = ?"
		args = append(args, filter.AccountID)
	}
	if filter.LabelID != "" {
		query += ` AND EXISTS (
  SELECT 1 FROM message_labels ml
  WHERE ml.account_id = m.account_id AND ml.message_id = m.id AND lower(ml.label_id) = lower(?)
)`
		args = append(args, filter.LabelID)
	}
	if filter.OlderThanDays > 0 {
		query += " AND COALESCE(m.internal_date, m.date, m.cached_at) < ?"
		args = append(args, now.AddDate(0, 0, -filter.OlderThanDays))
	}
	query += " ORDER BY m.account_id, m.id"

	rows := []purgeCandidate{}
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("plan cache purge candidates: %w", err)
	}
	return rows, nil
}

func (s CachePurgeService) populatePurgePlanCounts(ctx context.Context, plan *CachePurgePlan) error {
	for accountID, ids := range keysByAccount(plan.MessageKeys) {
		var messageStats struct {
			MessageCount   int   `db:"message_count"`
			BodyCount      int   `db:"body_count"`
			EstimatedBytes int64 `db:"estimated_bytes"`
		}
		query, args, err := sqlx.In(`
SELECT COUNT(*) AS message_count,
       COALESCE(SUM(CASE WHEN body_plain IS NOT NULL OR body_html IS NOT NULL OR cached_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS body_count,
       COALESCE(SUM(size_bytes + LENGTH(COALESCE(body_plain, '')) + LENGTH(COALESCE(body_html, '')) + LENGTH(COALESCE(raw_headers, ''))), 0) AS estimated_bytes
FROM messages
WHERE account_id = ? AND id IN (?)
`, accountID, ids)
		if err != nil {
			return err
		}
		if err := s.db.GetContext(ctx, &messageStats, query, args...); err != nil {
			return fmt.Errorf("plan cache purge message counts: %w", err)
		}
		plan.MessageCount += messageStats.MessageCount
		plan.BodyCount += messageStats.BodyCount
		plan.EstimatedBytes += messageStats.EstimatedBytes

		var attachmentStats struct {
			AttachmentCount int   `db:"attachment_count"`
			AttachmentBytes int64 `db:"attachment_bytes"`
		}
		query, args, err = sqlx.In(`
SELECT COUNT(*) AS attachment_count,
       COALESCE(SUM(size_bytes), 0) AS attachment_bytes
FROM attachments
WHERE account_id = ? AND message_id IN (?)
`, accountID, ids)
		if err != nil {
			return err
		}
		if err := s.db.GetContext(ctx, &attachmentStats, query, args...); err != nil {
			return fmt.Errorf("plan cache purge attachment counts: %w", err)
		}
		plan.AttachmentCount += attachmentStats.AttachmentCount
		plan.EstimatedBytes += attachmentStats.AttachmentBytes
	}
	return nil
}

func (s CachePurgeService) attachmentPaths(ctx context.Context, keys []MessageKey) ([]string, error) {
	paths := []string{}
	for accountID, ids := range keysByAccount(keys) {
		query, args, err := sqlx.In(`
SELECT local_path
FROM attachments
WHERE account_id = ? AND message_id IN (?) AND local_path IS NOT NULL AND local_path != ''
`, accountID, ids)
		if err != nil {
			return nil, err
		}
		var accountPaths []string
		if err := s.db.SelectContext(ctx, &accountPaths, query, args...); err != nil {
			return nil, fmt.Errorf("list cache purge attachment paths: %w", err)
		}
		paths = append(paths, accountPaths...)
	}
	return paths, nil
}

func keysByAccount(keys []MessageKey) map[string][]string {
	byAccount := map[string][]string{}
	for _, key := range keys {
		byAccount[key.AccountID] = append(byAccount[key.AccountID], key.MessageID)
	}
	return byAccount
}
