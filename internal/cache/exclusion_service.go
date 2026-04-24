package cache

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrInvalidCacheExclusion = errors.New("invalid cache exclusion")

type CacheExclusionService struct {
	db         *sqlx.DB
	exclusions CacheExclusionRepository
}

type MessageKey struct {
	AccountID string `db:"account_id"`
	MessageID string `db:"message_id"`
}

type CacheExclusionPurgePlan struct {
	Exclusion       CacheExclusion
	MessageCount    int   `db:"message_count"`
	BodyCount       int   `db:"body_count"`
	AttachmentCount int   `db:"attachment_count"`
	EstimatedBytes  int64 `db:"estimated_bytes"`
	MessageKeys     []MessageKey
}

type CacheExclusionPurgeResult struct {
	Plan            CacheExclusionPurgePlan
	DeletedMessages int
}

func NewCacheExclusionService(db *sqlx.DB) CacheExclusionService {
	return CacheExclusionService{
		db:         db,
		exclusions: NewCacheExclusionRepository(db),
	}
}

func (s CacheExclusionService) Add(ctx context.Context, exclusion CacheExclusion) (CacheExclusion, error) {
	normalized, err := NormalizeCacheExclusion(exclusion)
	if err != nil {
		return CacheExclusion{}, err
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now().UTC()
	}
	if err := s.exclusions.Upsert(ctx, normalized); err != nil {
		return CacheExclusion{}, err
	}
	return normalized, nil
}

func (s CacheExclusionService) PreviewPurge(ctx context.Context, exclusion CacheExclusion) (CacheExclusionPurgePlan, error) {
	normalized, err := NormalizeCacheExclusion(exclusion)
	if err != nil {
		return CacheExclusionPurgePlan{}, err
	}
	keys, err := s.matchingMessageKeys(ctx, normalized)
	if err != nil {
		return CacheExclusionPurgePlan{}, err
	}
	plan := CacheExclusionPurgePlan{Exclusion: normalized, MessageKeys: keys}
	if len(keys) == 0 {
		return plan, nil
	}

	ids := messageIDs(keys)
	messageQuery, args, err := sqlx.In(`
SELECT
  COUNT(*) AS message_count,
  COALESCE(SUM(CASE WHEN body_plain IS NOT NULL OR body_html IS NOT NULL OR cached_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS body_count,
  COALESCE(SUM(size_bytes + LENGTH(COALESCE(body_plain, '')) + LENGTH(COALESCE(body_html, '')) + LENGTH(COALESCE(raw_headers, ''))), 0) AS estimated_bytes
FROM messages
WHERE account_id = ? AND id IN (?)
`, normalized.AccountID, ids)
	if err != nil {
		return CacheExclusionPurgePlan{}, err
	}
	if err := s.db.GetContext(ctx, &plan, messageQuery, args...); err != nil {
		return CacheExclusionPurgePlan{}, fmt.Errorf("preview cache exclusion messages: %w", err)
	}
	plan.Exclusion = normalized
	plan.MessageKeys = keys

	var attachments struct {
		AttachmentCount int   `db:"attachment_count"`
		AttachmentBytes int64 `db:"attachment_bytes"`
	}
	attachmentQuery, args, err := sqlx.In(`
SELECT COUNT(*) AS attachment_count,
       COALESCE(SUM(size_bytes), 0) AS attachment_bytes
FROM attachments
WHERE account_id = ? AND message_id IN (?)
`, normalized.AccountID, ids)
	if err != nil {
		return CacheExclusionPurgePlan{}, err
	}
	if err := s.db.GetContext(ctx, &attachments, attachmentQuery, args...); err != nil {
		return CacheExclusionPurgePlan{}, fmt.Errorf("preview cache exclusion attachments: %w", err)
	}
	plan.AttachmentCount = attachments.AttachmentCount
	plan.EstimatedBytes += attachments.AttachmentBytes
	return plan, nil
}

func (s CacheExclusionService) ConfirmPurge(ctx context.Context, exclusion CacheExclusion) (CacheExclusionPurgeResult, error) {
	normalized, err := s.Add(ctx, exclusion)
	if err != nil {
		return CacheExclusionPurgeResult{}, err
	}
	plan, err := s.PreviewPurge(ctx, normalized)
	if err != nil {
		return CacheExclusionPurgeResult{}, err
	}
	if len(plan.MessageKeys) == 0 {
		return CacheExclusionPurgeResult{Plan: plan}, nil
	}

	query, args, err := sqlx.In("DELETE FROM messages WHERE account_id = ? AND id IN (?)", normalized.AccountID, messageIDs(plan.MessageKeys))
	if err != nil {
		return CacheExclusionPurgeResult{}, err
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return CacheExclusionPurgeResult{}, fmt.Errorf("purge cache exclusion matches: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return CacheExclusionPurgeResult{}, fmt.Errorf("read cache exclusion purge count: %w", err)
	}
	return CacheExclusionPurgeResult{Plan: plan, DeletedMessages: int(affected)}, nil
}

func NormalizeCacheExclusion(exclusion CacheExclusion) (CacheExclusion, error) {
	exclusion.AccountID = strings.TrimSpace(exclusion.AccountID)
	if exclusion.AccountID == "" {
		return CacheExclusion{}, invalidCacheExclusion("account id is required")
	}
	exclusion.MatchType = strings.ToLower(strings.TrimSpace(exclusion.MatchType))
	exclusion.MatchValue = strings.TrimSpace(exclusion.MatchValue)
	if exclusion.MatchValue == "" {
		return CacheExclusion{}, invalidCacheExclusion("match value is required")
	}
	switch exclusion.MatchType {
	case "sender":
		exclusion.MatchValue = senderAddressForCache(exclusion.MatchValue)
	case "domain":
		exclusion.MatchValue = strings.TrimPrefix(strings.ToLower(exclusion.MatchValue), "@")
	case "label":
	default:
		return CacheExclusion{}, invalidCacheExclusion("unsupported match type %q", exclusion.MatchType)
	}
	return exclusion, nil
}

func (s CacheExclusionService) matchingMessageKeys(ctx context.Context, exclusion CacheExclusion) ([]MessageKey, error) {
	switch exclusion.MatchType {
	case "label":
		keys := []MessageKey{}
		if err := s.db.SelectContext(ctx, &keys, `
SELECT DISTINCT ml.account_id AS account_id, ml.message_id AS message_id
FROM message_labels ml
WHERE ml.account_id = ? AND lower(ml.label_id) = lower(?)
ORDER BY ml.message_id
`, exclusion.AccountID, exclusion.MatchValue); err != nil {
			return nil, fmt.Errorf("find label exclusion matches: %w", err)
		}
		return keys, nil
	case "sender", "domain":
		rows := []struct {
			AccountID string `db:"account_id"`
			MessageID string `db:"message_id"`
			FromAddr  string `db:"from_addr"`
		}{}
		if err := s.db.SelectContext(ctx, &rows, `
SELECT account_id, id AS message_id, COALESCE(from_addr, '') AS from_addr
FROM messages
WHERE account_id = ?
ORDER BY id
`, exclusion.AccountID); err != nil {
			return nil, fmt.Errorf("find sender exclusion candidates: %w", err)
		}
		keys := make([]MessageKey, 0, len(rows))
		for _, row := range rows {
			if exclusionMatchesSender(exclusion, row.FromAddr) {
				keys = append(keys, MessageKey{AccountID: row.AccountID, MessageID: row.MessageID})
			}
		}
		return keys, nil
	default:
		return nil, invalidCacheExclusion("unsupported match type %q", exclusion.MatchType)
	}
}

func exclusionMatchesSender(exclusion CacheExclusion, from string) bool {
	address := senderAddressForCache(from)
	switch exclusion.MatchType {
	case "sender":
		return address == exclusion.MatchValue || strings.EqualFold(strings.TrimSpace(from), exclusion.MatchValue)
	case "domain":
		_, domain, ok := strings.Cut(address, "@")
		return ok && domain == exclusion.MatchValue
	default:
		return false
	}
}

func senderAddressForCache(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err == nil {
		return strings.ToLower(address.Address)
	}
	return strings.ToLower(value)
}

func messageIDs(keys []MessageKey) []string {
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.MessageID)
	}
	return ids
}

func invalidCacheExclusion(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCacheExclusion, fmt.Sprintf(format, args...))
}
