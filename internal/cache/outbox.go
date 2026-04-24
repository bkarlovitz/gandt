package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	OutboxStatusPending = "pending"
	OutboxStatusSent    = "sent"
	OutboxStatusFailed  = "failed"
)

var ErrOutboxMessageNotFound = errors.New("outbox message not found")

type OutboxMessage struct {
	ID        string
	AccountID string
	RawRFC822 []byte
	QueuedAt  time.Time
	Attempts  int
	LastError string
	Status    string
}

type OutboxRepository struct {
	db *sqlx.DB
}

func NewOutboxRepository(db *sqlx.DB) OutboxRepository {
	return OutboxRepository{db: db}
}

func (r OutboxRepository) Queue(ctx context.Context, message OutboxMessage) (OutboxMessage, error) {
	if message.AccountID == "" {
		return OutboxMessage{}, errors.New("outbox account id is required")
	}
	if len(message.RawRFC822) == 0 {
		return OutboxMessage{}, errors.New("outbox raw message is required")
	}
	if message.QueuedAt.IsZero() {
		message.QueuedAt = time.Now().UTC()
	}
	if message.ID == "" {
		message.ID = fmt.Sprintf("%s-%d", message.AccountID, message.QueuedAt.UnixNano())
	}
	if message.Status == "" {
		message.Status = OutboxStatusPending
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO outbox (id, account_id, raw_rfc822, queued_at, attempts, last_error, status)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, message.ID, message.AccountID, message.RawRFC822, message.QueuedAt, message.Attempts, nullIfEmpty(message.LastError), message.Status)
	if err != nil {
		return OutboxMessage{}, fmt.Errorf("queue outbox message: %w", err)
	}
	return message, nil
}

func (r OutboxRepository) Pending(ctx context.Context, accountID string, limit int) ([]OutboxMessage, error) {
	query := `
SELECT id, account_id, raw_rfc822, queued_at, attempts, last_error, status
FROM outbox
WHERE account_id = ? AND status = ?
ORDER BY queued_at, id
`
	args := []any{accountID, OutboxStatusPending}
	if limit > 0 {
		query += "LIMIT ?"
		args = append(args, limit)
	}
	var rows []outboxRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list pending outbox: %w", err)
	}
	out := make([]OutboxMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.message())
	}
	return out, nil
}

func (r OutboxRepository) MarkSent(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE outbox
SET status = ?, last_error = NULL
WHERE id = ?
`, OutboxStatusSent, id)
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}
	return requireAffected(result, ErrOutboxMessageNotFound)
}

func (r OutboxRepository) MarkRetry(ctx context.Context, id string, errText string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE outbox
SET attempts = attempts + 1, last_error = ?, status = ?
WHERE id = ?
`, errText, OutboxStatusPending, id)
	if err != nil {
		return fmt.Errorf("mark outbox retry: %w", err)
	}
	return requireAffected(result, ErrOutboxMessageNotFound)
}

func (r OutboxRepository) MarkFailed(ctx context.Context, id string, errText string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE outbox
SET attempts = attempts + 1, last_error = ?, status = ?
WHERE id = ?
`, errText, OutboxStatusFailed, id)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return requireAffected(result, ErrOutboxMessageNotFound)
}

func (r OutboxRepository) Get(ctx context.Context, id string) (OutboxMessage, error) {
	var row outboxRow
	err := r.db.GetContext(ctx, &row, `
SELECT id, account_id, raw_rfc822, queued_at, attempts, last_error, status
FROM outbox
WHERE id = ?
`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return OutboxMessage{}, ErrOutboxMessageNotFound
	}
	if err != nil {
		return OutboxMessage{}, fmt.Errorf("get outbox message: %w", err)
	}
	return row.message(), nil
}

type outboxRow struct {
	ID        string         `db:"id"`
	AccountID string         `db:"account_id"`
	RawRFC822 []byte         `db:"raw_rfc822"`
	QueuedAt  time.Time      `db:"queued_at"`
	Attempts  int            `db:"attempts"`
	LastError sql.NullString `db:"last_error"`
	Status    string         `db:"status"`
}

func (r outboxRow) message() OutboxMessage {
	message := OutboxMessage{
		ID:        r.ID,
		AccountID: r.AccountID,
		RawRFC822: append([]byte{}, r.RawRFC822...),
		QueuedAt:  r.QueuedAt,
		Attempts:  r.Attempts,
		Status:    r.Status,
	}
	if r.LastError.Valid {
		message.LastError = r.LastError.String
	}
	return message
}
