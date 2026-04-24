package cache

import (
	"context"
	"fmt"
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
	byAccount := map[string][]string{}
	for _, key := range plan.MessageKeys {
		byAccount[key.AccountID] = append(byAccount[key.AccountID], key.MessageID)
	}
	for accountID, ids := range byAccount {
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
