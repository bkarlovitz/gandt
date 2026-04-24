package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

var ErrLabelNotFound = errors.New("label not found")

type Label struct {
	AccountID string
	ID        string
	Name      string
	Type      string
	Unread    int
	Total     int
	ColorBG   string
	ColorFG   string
}

type LabelRepository struct {
	db *sqlx.DB
}

func NewLabelRepository(db *sqlx.DB) LabelRepository {
	return LabelRepository{db: db}
}

func (r LabelRepository) Upsert(ctx context.Context, label Label) error {
	if strings.TrimSpace(label.AccountID) == "" {
		return errors.New("label account id is required")
	}
	if strings.TrimSpace(label.ID) == "" {
		return errors.New("label id is required")
	}
	if strings.TrimSpace(label.Name) == "" {
		return errors.New("label name is required")
	}
	if strings.TrimSpace(label.Type) == "" {
		return errors.New("label type is required")
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO labels (account_id, id, name, type, unread, total, color_bg, color_fg)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, id) DO UPDATE SET
  name = excluded.name,
  type = excluded.type,
  unread = excluded.unread,
  total = excluded.total,
  color_bg = excluded.color_bg,
  color_fg = excluded.color_fg
`, label.AccountID, label.ID, label.Name, label.Type, label.Unread, label.Total, nullIfEmpty(label.ColorBG), nullIfEmpty(label.ColorFG))
	if err != nil {
		return fmt.Errorf("upsert label: %w", err)
	}
	return nil
}

func (r LabelRepository) List(ctx context.Context, accountID string) ([]Label, error) {
	rows := []labelRow{}
	if err := r.db.SelectContext(ctx, &rows, `
SELECT account_id, id, name, type, unread, total, color_bg, color_fg
FROM labels
WHERE account_id = ?
ORDER BY type DESC, name COLLATE NOCASE
`, accountID); err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}

	labels := make([]Label, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.label())
	}
	return labels, nil
}

func (r LabelRepository) Delete(ctx context.Context, accountID string, labelID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM labels WHERE account_id = ? AND id = ?", accountID, labelID)
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return requireAffected(result, ErrLabelNotFound)
}

type labelRow struct {
	AccountID string         `db:"account_id"`
	ID        string         `db:"id"`
	Name      string         `db:"name"`
	Type      string         `db:"type"`
	Unread    int            `db:"unread"`
	Total     int            `db:"total"`
	ColorBG   sql.NullString `db:"color_bg"`
	ColorFG   sql.NullString `db:"color_fg"`
}

func (row labelRow) label() Label {
	return Label{
		AccountID: row.AccountID,
		ID:        row.ID,
		Name:      row.Name,
		Type:      row.Type,
		Unread:    row.Unread,
		Total:     row.Total,
		ColorBG:   row.ColorBG.String,
		ColorFG:   row.ColorFG.String,
	}
}
