package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrCacheExclusionNotFound = errors.New("cache exclusion not found")

type CacheExclusion struct {
	AccountID  string
	MatchType  string
	MatchValue string
	CreatedAt  time.Time
}

type CacheExclusionRepository struct {
	db *sqlx.DB
}

func NewCacheExclusionRepository(db *sqlx.DB) CacheExclusionRepository {
	return CacheExclusionRepository{db: db}
}

func (r CacheExclusionRepository) Upsert(ctx context.Context, exclusion CacheExclusion) error {
	if strings.TrimSpace(exclusion.AccountID) == "" {
		return errors.New("cache exclusion account id is required")
	}
	matchType := strings.ToLower(strings.TrimSpace(exclusion.MatchType))
	if matchType == "" {
		return errors.New("cache exclusion match type is required")
	}
	matchValue := strings.TrimSpace(exclusion.MatchValue)
	if matchValue == "" {
		return errors.New("cache exclusion match value is required")
	}
	createdAt := exclusion.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO cache_exclusions (account_id, match_type, match_value, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, match_type, match_value) DO UPDATE SET
  created_at = excluded.created_at
`, exclusion.AccountID, matchType, matchValue, createdAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert cache exclusion: %w", err)
	}
	return nil
}

func (r CacheExclusionRepository) List(ctx context.Context, accountID string) ([]CacheExclusion, error) {
	rows := []cacheExclusionRow{}
	if err := r.db.SelectContext(ctx, &rows, `
SELECT account_id, match_type, match_value, created_at
FROM cache_exclusions
WHERE account_id = ?
ORDER BY match_type, match_value
`, accountID); err != nil {
		return nil, fmt.Errorf("list cache exclusions: %w", err)
	}

	exclusions := make([]CacheExclusion, 0, len(rows))
	for _, row := range rows {
		exclusions = append(exclusions, row.exclusion())
	}
	return exclusions, nil
}

func (r CacheExclusionRepository) Delete(ctx context.Context, accountID string, matchType string, matchValue string) error {
	result, err := r.db.ExecContext(ctx, `
DELETE FROM cache_exclusions
WHERE account_id = ? AND match_type = ? AND match_value = ?
`, accountID, strings.ToLower(strings.TrimSpace(matchType)), strings.TrimSpace(matchValue))
	if err != nil {
		return fmt.Errorf("delete cache exclusion: %w", err)
	}
	return requireAffected(result, ErrCacheExclusionNotFound)
}

type cacheExclusionRow struct {
	AccountID  string    `db:"account_id"`
	MatchType  string    `db:"match_type"`
	MatchValue string    `db:"match_value"`
	CreatedAt  time.Time `db:"created_at"`
}

func (row cacheExclusionRow) exclusion() CacheExclusion {
	return CacheExclusion{
		AccountID:  row.AccountID,
		MatchType:  row.MatchType,
		MatchValue: row.MatchValue,
		CreatedAt:  row.CreatedAt,
	}
}
