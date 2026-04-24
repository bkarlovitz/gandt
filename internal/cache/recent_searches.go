package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type RecentSearch struct {
	AccountID string    `db:"account_id"`
	Query     string    `db:"query"`
	Mode      string    `db:"mode"`
	LastUsed  time.Time `db:"last_used"`
}

type RecentSearchRepository struct {
	db *sqlx.DB
}

func NewRecentSearchRepository(db *sqlx.DB) RecentSearchRepository {
	return RecentSearchRepository{db: db}
}

func (r RecentSearchRepository) Record(ctx context.Context, search RecentSearch, limit int) error {
	search.Query = strings.TrimSpace(search.Query)
	search.Mode = strings.TrimSpace(search.Mode)
	if strings.TrimSpace(search.AccountID) == "" {
		return errors.New("recent search account id is required")
	}
	if search.Query == "" {
		return errors.New("recent search query is required")
	}
	if search.Mode == "" {
		return errors.New("recent search mode is required")
	}
	if search.LastUsed.IsZero() {
		search.LastUsed = time.Now().UTC()
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recent search record: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO recent_searches (account_id, query, mode, last_used)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, query, mode) DO UPDATE SET
  last_used = excluded.last_used
`, search.AccountID, search.Query, search.Mode, search.LastUsed.UTC()); err != nil {
		return fmt.Errorf("record recent search: %w", err)
	}
	if limit > 0 {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM recent_searches
WHERE account_id = ? AND (query, mode) NOT IN (
  SELECT query, mode
  FROM recent_searches
  WHERE account_id = ?
  ORDER BY last_used DESC, query, mode
  LIMIT ?
)
`, search.AccountID, search.AccountID, limit); err != nil {
			return fmt.Errorf("trim recent searches: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recent search record: %w", err)
	}
	return nil
}

func (r RecentSearchRepository) List(ctx context.Context, accountID string, limit int) ([]RecentSearch, error) {
	query := `
SELECT account_id, query, mode, last_used
FROM recent_searches
WHERE account_id = ?
ORDER BY last_used DESC, query, mode
`
	args := []any{accountID}
	if limit > 0 {
		query += "LIMIT ?"
		args = append(args, limit)
	}
	searches := []RecentSearch{}
	if err := r.db.SelectContext(ctx, &searches, query, args...); err != nil {
		return nil, fmt.Errorf("list recent searches: %w", err)
	}
	return searches, nil
}

func (r RecentSearchRepository) Delete(ctx context.Context, accountID string, query string, mode string) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM recent_searches
WHERE account_id = ? AND query = ? AND mode = ?
`, accountID, strings.TrimSpace(query), strings.TrimSpace(mode))
	if err != nil {
		return fmt.Errorf("delete recent search: %w", err)
	}
	return nil
}
