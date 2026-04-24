package cache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSyncPolicyEditorSaveUpdateResetAndDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	account, err := NewAccountRepository(db).Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	editor := NewSyncPolicyEditor(db)

	retention := 180
	maxMB := 25
	saved, err := editor.Save(ctx, SyncPolicy{
		AccountID:       account.ID,
		LabelID:         "Label_1",
		Include:         true,
		Depth:           "full",
		RetentionDays:   &retention,
		AttachmentRule:  "under_size",
		AttachmentMaxMB: &maxMB,
	})
	if err != nil {
		t.Fatalf("save policy: %v", err)
	}
	if saved.Explicit == nil || saved.Explicit.UpdatedAt.IsZero() {
		t.Fatalf("saved explicit policy = %#v, want stamped explicit row", saved.Explicit)
	}
	if saved.Effective.LabelID != "Label_1" || saved.Effective.Depth != "full" || valueOrZero(saved.Effective.AttachmentMaxMB) != 25 {
		t.Fatalf("saved effective policy = %#v, want explicit full under_size", saved.Effective)
	}

	retention = 30
	updatedAt := time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC)
	updated, err := editor.Save(ctx, SyncPolicy{
		AccountID:      account.ID,
		LabelID:        "Label_1",
		Include:        true,
		Depth:          "body",
		RetentionDays:  &retention,
		AttachmentRule: "none",
		UpdatedAt:      updatedAt,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if updated.Effective.Depth != "body" || valueOrZero(updated.Effective.RetentionDays) != 30 || !updated.Effective.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated policy = %#v, want body/30 with explicit timestamp", updated.Effective)
	}

	reset, err := editor.ResetToDefault(ctx, account.ID, "Label_1")
	if err != nil {
		t.Fatalf("reset policy: %v", err)
	}
	if reset.Explicit != nil || reset.Effective.LabelID != DefaultPolicyLabelID || reset.Effective.Depth != "metadata" {
		t.Fatalf("reset result = %#v, want account default metadata", reset)
	}
	if _, err := NewSyncPolicyRepository(db).Get(ctx, account.ID, "Label_1"); !errors.Is(err, ErrSyncPolicyNotFound) {
		t.Fatalf("get reset explicit row error = %v, want ErrSyncPolicyNotFound", err)
	}

	if _, err := editor.Save(ctx, SyncPolicy{AccountID: account.ID, LabelID: "Label_2", Include: true, Depth: "metadata", AttachmentRule: "none"}); err != nil {
		t.Fatalf("save second policy: %v", err)
	}
	deleted, err := editor.DeleteExplicit(ctx, account.ID, "Label_2")
	if err != nil {
		t.Fatalf("delete explicit: %v", err)
	}
	if deleted.Effective.LabelID != DefaultPolicyLabelID {
		t.Fatalf("delete result = %#v, want effective default", deleted)
	}
}

func TestValidateSyncPolicyRejectsInvalidCombinations(t *testing.T) {
	validMax := 10
	tests := map[string]SyncPolicy{
		"missing account":        {LabelID: "INBOX", Include: true, Depth: "metadata", AttachmentRule: "none"},
		"bad depth":              {AccountID: "acct", LabelID: "INBOX", Include: true, Depth: "everything", AttachmentRule: "none"},
		"positive retention":     {AccountID: "acct", LabelID: "INBOX", Include: true, Depth: "metadata", RetentionDays: intPtr(-1), AttachmentRule: "none"},
		"include mismatch":       {AccountID: "acct", LabelID: "INBOX", Include: false, Depth: "metadata", AttachmentRule: "none"},
		"missing under size max": {AccountID: "acct", LabelID: "INBOX", Include: true, Depth: "full", AttachmentRule: "under_size"},
		"max with all":           {AccountID: "acct", LabelID: "INBOX", Include: true, Depth: "full", AttachmentRule: "all", AttachmentMaxMB: &validMax},
	}

	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateSyncPolicy(policy)
			if !errors.Is(err, ErrInvalidSyncPolicy) {
				t.Fatalf("validate error = %v, want ErrInvalidSyncPolicy", err)
			}
			if err == nil || !strings.Contains(err.Error(), "invalid sync policy") {
				t.Fatalf("validate error text = %v, want contextual invalid policy error", err)
			}
		})
	}
}
