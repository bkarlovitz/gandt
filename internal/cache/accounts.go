package cache

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrDuplicateEmail  = errors.New("account email already exists")
)

type Account struct {
	ID          string
	Email       string
	DisplayName string
	AddedAt     time.Time
	LastSyncAt  *time.Time
	HistoryID   string
	Color       string
}

type CreateAccountParams struct {
	Email       string
	DisplayName string
	HistoryID   string
	Color       string
}

type AccountRepository struct {
	db *sqlx.DB
}

func NewAccountRepository(db *sqlx.DB) AccountRepository {
	return AccountRepository{db: db}
}

func (r AccountRepository) Create(ctx context.Context, params CreateAccountParams) (Account, error) {
	email := strings.TrimSpace(params.Email)
	if email == "" {
		return Account{}, errors.New("account email is required")
	}

	account := Account{
		ID:          newOpaqueID(),
		Email:       email,
		DisplayName: strings.TrimSpace(params.DisplayName),
		AddedAt:     time.Now().UTC(),
		HistoryID:   strings.TrimSpace(params.HistoryID),
		Color:       strings.TrimSpace(params.Color),
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("begin create account: %w", err)
	}
	defer tx.Rollback()

	var duplicate int
	if err := tx.GetContext(ctx, &duplicate, "SELECT COUNT(*) FROM accounts WHERE lower(email) = lower(?)", account.Email); err != nil {
		return Account{}, fmt.Errorf("check duplicate account email: %w", err)
	}
	if duplicate > 0 {
		return Account{}, ErrDuplicateEmail
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO accounts (id, email, display_name, added_at, history_id, color)
VALUES (?, ?, ?, ?, ?, ?)
`, account.ID, account.Email, nullIfEmpty(account.DisplayName), account.AddedAt, nullIfEmpty(account.HistoryID), nullIfEmpty(account.Color))
	if isUniqueConstraint(err) {
		return Account{}, ErrDuplicateEmail
	}
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	if err := seedDefaultSyncPolicies(ctx, tx, account.ID, account.AddedAt); err != nil {
		return Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("commit create account: %w", err)
	}

	return account, nil
}

func (r AccountRepository) Get(ctx context.Context, id string) (Account, error) {
	return r.get(ctx, "WHERE id = ?", id)
}

func (r AccountRepository) GetByEmail(ctx context.Context, email string) (Account, error) {
	return r.get(ctx, "WHERE lower(email) = lower(?)", strings.TrimSpace(email))
}

func (r AccountRepository) List(ctx context.Context) ([]Account, error) {
	rows := []accountRow{}
	if err := r.db.SelectContext(ctx, &rows, `
SELECT id, email, display_name, added_at, last_sync_at, history_id, color
FROM accounts
ORDER BY added_at, email
`); err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	accounts := make([]Account, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, row.account())
	}
	return accounts, nil
}

func (r AccountRepository) UpdateSyncMetadata(ctx context.Context, id string, historyID string, syncedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE accounts
SET history_id = ?, last_sync_at = ?
WHERE id = ?
`, nullIfEmpty(strings.TrimSpace(historyID)), syncedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("update account sync metadata: %w", err)
	}
	return requireAffected(result, ErrAccountNotFound)
}

func (r AccountRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return requireAffected(result, ErrAccountNotFound)
}

func (r AccountRepository) get(ctx context.Context, clause string, arg any) (Account, error) {
	var row accountRow
	err := r.db.GetContext(ctx, &row, `
SELECT id, email, display_name, added_at, last_sync_at, history_id, color
FROM accounts
`+clause, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	return row.account(), nil
}

type accountRow struct {
	ID          string         `db:"id"`
	Email       string         `db:"email"`
	DisplayName sql.NullString `db:"display_name"`
	AddedAt     time.Time      `db:"added_at"`
	LastSyncAt  sql.NullTime   `db:"last_sync_at"`
	HistoryID   sql.NullString `db:"history_id"`
	Color       sql.NullString `db:"color"`
}

func (row accountRow) account() Account {
	account := Account{
		ID:          row.ID,
		Email:       row.Email,
		DisplayName: row.DisplayName.String,
		AddedAt:     row.AddedAt,
		HistoryID:   row.HistoryID.String,
		Color:       row.Color.String,
	}
	if row.LastSyncAt.Valid {
		account.LastSyncAt = &row.LastSyncAt.Time
	}
	return account
}

func newOpaqueID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate account id: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func requireAffected(result sql.Result, notFound error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected row count: %w", err)
	}
	if affected == 0 {
		return notFound
	}
	return nil
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "constraint failed") && strings.Contains(msg, "UNIQUE")
}
