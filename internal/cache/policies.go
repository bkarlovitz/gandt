package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

const DefaultPolicyLabelID = "*"

var ErrSyncPolicyNotFound = errors.New("sync policy not found")

type SyncPolicy struct {
	AccountID       string
	LabelID         string
	Include         bool
	Depth           string
	RetentionDays   *int
	AttachmentRule  string
	AttachmentMaxMB *int
	UpdatedAt       time.Time
}

type SyncPolicyRepository struct {
	db *sqlx.DB
}

func NewSyncPolicyRepository(db *sqlx.DB) SyncPolicyRepository {
	return SyncPolicyRepository{db: db}
}

func (r SyncPolicyRepository) Upsert(ctx context.Context, policy SyncPolicy) error {
	return upsertSyncPolicy(ctx, r.db, policy)
}

func (r SyncPolicyRepository) Get(ctx context.Context, accountID string, labelID string) (SyncPolicy, error) {
	return getSyncPolicy(ctx, r.db, accountID, labelID)
}

func (r SyncPolicyRepository) EffectiveForLabel(ctx context.Context, accountID string, labelID string) (SyncPolicy, error) {
	policy, err := r.Get(ctx, accountID, labelID)
	if err == nil {
		return policy, nil
	}
	if !errors.Is(err, ErrSyncPolicyNotFound) {
		return SyncPolicy{}, err
	}
	return r.Get(ctx, accountID, DefaultPolicyLabelID)
}

func (r SyncPolicyRepository) List(ctx context.Context, accountID string) ([]SyncPolicy, error) {
	rows := []syncPolicyRow{}
	if err := r.db.SelectContext(ctx, &rows, `
SELECT account_id, label_id, include, depth, retention_days, attachment_rule, attachment_max_mb, updated_at
FROM sync_policies
WHERE account_id = ?
ORDER BY label_id
`, accountID); err != nil {
		return nil, fmt.Errorf("list sync policies: %w", err)
	}

	policies := make([]SyncPolicy, 0, len(rows))
	for _, row := range rows {
		policies = append(policies, row.policy())
	}
	return policies, nil
}

func (r SyncPolicyRepository) Delete(ctx context.Context, accountID string, labelID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM sync_policies WHERE account_id = ? AND label_id = ?", accountID, labelID)
	if err != nil {
		return fmt.Errorf("delete sync policy: %w", err)
	}
	return requireAffected(result, ErrSyncPolicyNotFound)
}

func seedDefaultSyncPolicies(ctx context.Context, exec sqlx.ExtContext, accountID string, updatedAt time.Time) error {
	for _, policy := range defaultSyncPolicies(accountID, updatedAt) {
		if err := upsertSyncPolicy(ctx, exec, policy); err != nil {
			return fmt.Errorf("seed default sync policy %s: %w", policy.LabelID, err)
		}
	}
	return nil
}

func defaultSyncPolicies(accountID string, updatedAt time.Time) []SyncPolicy {
	underSizeMB := 10
	return []SyncPolicy{
		{AccountID: accountID, LabelID: "INBOX", Include: true, Depth: "full", RetentionDays: intPtr(90), AttachmentRule: "under_size", AttachmentMaxMB: &underSizeMB, UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "STARRED", Include: true, Depth: "full", RetentionDays: intPtr(90), AttachmentRule: "under_size", AttachmentMaxMB: &underSizeMB, UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "IMPORTANT", Include: true, Depth: "full", RetentionDays: intPtr(90), AttachmentRule: "under_size", AttachmentMaxMB: &underSizeMB, UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "SENT", Include: true, Depth: "body", RetentionDays: intPtr(90), AttachmentRule: "none", UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "DRAFT", Include: true, Depth: "body", RetentionDays: intPtr(90), AttachmentRule: "none", UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "SPAM", Include: true, Depth: "metadata", RetentionDays: intPtr(30), AttachmentRule: "none", UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: "TRASH", Include: true, Depth: "metadata", RetentionDays: intPtr(30), AttachmentRule: "none", UpdatedAt: updatedAt},
		{AccountID: accountID, LabelID: DefaultPolicyLabelID, Include: true, Depth: "metadata", RetentionDays: intPtr(365), AttachmentRule: "none", UpdatedAt: updatedAt},
	}
}

func upsertSyncPolicy(ctx context.Context, exec sqlx.ExtContext, policy SyncPolicy) error {
	if policy.AccountID == "" {
		return errors.New("sync policy account id is required")
	}
	if policy.LabelID == "" {
		return errors.New("sync policy label id is required")
	}
	if policy.Depth == "" {
		return errors.New("sync policy depth is required")
	}
	if policy.AttachmentRule == "" {
		return errors.New("sync policy attachment rule is required")
	}
	updatedAt := policy.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	_, err := exec.ExecContext(ctx, `
INSERT INTO sync_policies (account_id, label_id, include, depth, retention_days, attachment_rule, attachment_max_mb, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, label_id) DO UPDATE SET
  include = excluded.include,
  depth = excluded.depth,
  retention_days = excluded.retention_days,
  attachment_rule = excluded.attachment_rule,
  attachment_max_mb = excluded.attachment_max_mb,
  updated_at = excluded.updated_at
`, policy.AccountID, policy.LabelID, boolInt(policy.Include), policy.Depth, intOrNil(policy.RetentionDays), policy.AttachmentRule, intOrNil(policy.AttachmentMaxMB), updatedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert sync policy: %w", err)
	}
	return nil
}

func getSyncPolicy(ctx context.Context, db *sqlx.DB, accountID string, labelID string) (SyncPolicy, error) {
	var row syncPolicyRow
	err := db.GetContext(ctx, &row, `
SELECT account_id, label_id, include, depth, retention_days, attachment_rule, attachment_max_mb, updated_at
FROM sync_policies
WHERE account_id = ? AND label_id = ?
`, accountID, labelID)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncPolicy{}, ErrSyncPolicyNotFound
	}
	if err != nil {
		return SyncPolicy{}, fmt.Errorf("get sync policy: %w", err)
	}
	return row.policy(), nil
}

type syncPolicyRow struct {
	AccountID       string        `db:"account_id"`
	LabelID         string        `db:"label_id"`
	Include         int           `db:"include"`
	Depth           string        `db:"depth"`
	RetentionDays   sql.NullInt64 `db:"retention_days"`
	AttachmentRule  string        `db:"attachment_rule"`
	AttachmentMaxMB sql.NullInt64 `db:"attachment_max_mb"`
	UpdatedAt       time.Time     `db:"updated_at"`
}

func (row syncPolicyRow) policy() SyncPolicy {
	return SyncPolicy{
		AccountID:       row.AccountID,
		LabelID:         row.LabelID,
		Include:         row.Include == 1,
		Depth:           row.Depth,
		RetentionDays:   nullIntPtr(row.RetentionDays),
		AttachmentRule:  row.AttachmentRule,
		AttachmentMaxMB: nullIntPtr(row.AttachmentMaxMB),
		UpdatedAt:       row.UpdatedAt,
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intPtr(value int) *int {
	return &value
}

func intOrNil(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	out := int(value.Int64)
	return &out
}
