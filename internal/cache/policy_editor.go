package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrInvalidSyncPolicy = errors.New("invalid sync policy")

type SyncPolicyEditor struct {
	policies SyncPolicyRepository
}

type SyncPolicyEditResult struct {
	Explicit  *SyncPolicy
	Effective SyncPolicy
}

func NewSyncPolicyEditor(db *sqlx.DB) SyncPolicyEditor {
	return SyncPolicyEditor{policies: NewSyncPolicyRepository(db)}
}

func (e SyncPolicyEditor) Save(ctx context.Context, policy SyncPolicy) (SyncPolicyEditResult, error) {
	if policy.UpdatedAt.IsZero() {
		policy.UpdatedAt = time.Now().UTC()
	}
	if err := ValidateSyncPolicy(policy); err != nil {
		return SyncPolicyEditResult{}, err
	}
	if err := e.policies.Upsert(ctx, policy); err != nil {
		return SyncPolicyEditResult{}, err
	}
	effective, err := e.policies.EffectiveForLabel(ctx, policy.AccountID, policy.LabelID)
	if err != nil {
		return SyncPolicyEditResult{}, err
	}
	return SyncPolicyEditResult{Explicit: &policy, Effective: effective}, nil
}

func (e SyncPolicyEditor) DeleteExplicit(ctx context.Context, accountID string, labelID string) (SyncPolicyEditResult, error) {
	if err := e.policies.Delete(ctx, accountID, labelID); err != nil {
		return SyncPolicyEditResult{}, err
	}
	effective, err := e.policies.EffectiveForLabel(ctx, accountID, labelID)
	if errors.Is(err, ErrSyncPolicyNotFound) {
		return SyncPolicyEditResult{}, nil
	}
	if err != nil {
		return SyncPolicyEditResult{}, err
	}
	return SyncPolicyEditResult{Effective: effective}, nil
}

func (e SyncPolicyEditor) ResetToDefault(ctx context.Context, accountID string, labelID string) (SyncPolicyEditResult, error) {
	err := e.policies.Delete(ctx, accountID, labelID)
	if err != nil && !errors.Is(err, ErrSyncPolicyNotFound) {
		return SyncPolicyEditResult{}, err
	}
	effective, err := e.policies.EffectiveForLabel(ctx, accountID, labelID)
	if errors.Is(err, ErrSyncPolicyNotFound) {
		return SyncPolicyEditResult{}, nil
	}
	if err != nil {
		return SyncPolicyEditResult{}, err
	}
	return SyncPolicyEditResult{Effective: effective}, nil
}

func ValidateSyncPolicy(policy SyncPolicy) error {
	if policy.AccountID == "" {
		return invalidSyncPolicy("account id is required")
	}
	if policy.LabelID == "" {
		return invalidSyncPolicy("label id is required")
	}
	if policy.RetentionDays != nil && *policy.RetentionDays <= 0 {
		return invalidSyncPolicy("retention days must be positive")
	}
	if policy.AttachmentMaxMB != nil && *policy.AttachmentMaxMB <= 0 {
		return invalidSyncPolicy("attachment max must be positive")
	}

	switch policy.Depth {
	case "none":
		if policy.Include {
			return invalidSyncPolicy("include must be false for none depth")
		}
	case "metadata", "body", "full":
		if !policy.Include {
			return invalidSyncPolicy("include must be true when depth caches data")
		}
	default:
		return invalidSyncPolicy("unsupported depth %q", policy.Depth)
	}

	switch policy.AttachmentRule {
	case "none":
		if policy.AttachmentMaxMB != nil {
			return invalidSyncPolicy("attachment max requires under_size rule")
		}
	case "all":
		if policy.AttachmentMaxMB != nil {
			return invalidSyncPolicy("attachment max requires under_size rule")
		}
	case "under_size":
		if policy.AttachmentMaxMB == nil {
			return invalidSyncPolicy("under_size attachment rule requires a max")
		}
	default:
		return invalidSyncPolicy("unsupported attachment rule %q", policy.AttachmentRule)
	}

	return nil
}

func invalidSyncPolicy(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidSyncPolicy, fmt.Sprintf(format, args...))
}
