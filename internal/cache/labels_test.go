package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLabelRepositoryUpsertListDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accounts := NewAccountRepository(db)
	labels := NewLabelRepository(db)

	account, err := accounts.Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	label := Label{
		AccountID: account.ID,
		ID:        "Label_1",
		Name:      "Receipts",
		Type:      "user",
		Unread:    2,
		Total:     5,
		ColorBG:   "#111111",
		ColorFG:   "#eeeeee",
	}
	if err := labels.Upsert(ctx, label); err != nil {
		t.Fatalf("upsert label: %v", err)
	}

	label.Name = "Receipts 2026"
	label.Unread = 4
	if err := labels.Upsert(ctx, label); err != nil {
		t.Fatalf("upsert updated label: %v", err)
	}

	listed, err := labels.List(ctx, account.ID)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("labels = %#v, want one label", listed)
	}
	if listed[0].Name != "Receipts 2026" || listed[0].Unread != 4 || listed[0].ColorBG != "#111111" {
		t.Fatalf("listed label = %#v, want updated values", listed[0])
	}

	if err := labels.Delete(ctx, account.ID, "Label_1"); err != nil {
		t.Fatalf("delete label: %v", err)
	}
	if err := labels.Delete(ctx, account.ID, "Label_1"); !errors.Is(err, ErrLabelNotFound) {
		t.Fatalf("delete missing label error = %v, want ErrLabelNotFound", err)
	}
}

func TestLabelRepositoryListsSystemLabelsBeforeUserLabels(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accounts := NewAccountRepository(db)
	labels := NewLabelRepository(db)

	account, err := accounts.Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	for _, label := range []Label{
		{AccountID: account.ID, ID: "Label_1", Name: "Receipts", Type: "user"},
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "SENT", Name: "Sent", Type: "system"},
		{AccountID: account.ID, ID: "Label_2", Name: "Travel", Type: "user"},
	} {
		if err := labels.Upsert(ctx, label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}

	listed, err := labels.List(ctx, account.ID)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	got := labelIDs(listed)
	want := []string{"INBOX", "SENT", "Label_1", "Label_2"}
	if !equalStrings(got, want) {
		t.Fatalf("label order = %#v, want %#v", got, want)
	}
}

func TestAccountCreationSeedsDefaultSyncPolicies(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accounts := NewAccountRepository(db)
	policies := NewSyncPolicyRepository(db)

	account, err := accounts.Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	listed, err := policies.List(ctx, account.ID)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	if len(listed) != 8 {
		t.Fatalf("default policies = %d, want 8: %#v", len(listed), listed)
	}

	inbox, err := policies.Get(ctx, account.ID, "INBOX")
	if err != nil {
		t.Fatalf("get inbox policy: %v", err)
	}
	if inbox.Depth != "full" || valueOrZero(inbox.RetentionDays) != 90 || inbox.AttachmentRule != "under_size" || valueOrZero(inbox.AttachmentMaxMB) != 10 {
		t.Fatalf("inbox policy = %#v, want full/90/under_size/10", inbox)
	}

	spam, err := policies.Get(ctx, account.ID, "SPAM")
	if err != nil {
		t.Fatalf("get spam policy: %v", err)
	}
	if spam.Depth != "metadata" || valueOrZero(spam.RetentionDays) != 30 || spam.AttachmentRule != "none" {
		t.Fatalf("spam policy = %#v, want metadata/30/none", spam)
	}
}

func TestSyncPolicyRepositoryFallbackAndUpsert(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	accounts := NewAccountRepository(db)
	policies := NewSyncPolicyRepository(db)

	account, err := accounts.Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	fallback, err := policies.EffectiveForLabel(ctx, account.ID, "Label_1")
	if err != nil {
		t.Fatalf("get fallback policy: %v", err)
	}
	if fallback.LabelID != DefaultPolicyLabelID || fallback.Depth != "metadata" || valueOrZero(fallback.RetentionDays) != 365 {
		t.Fatalf("fallback policy = %#v, want account default", fallback)
	}

	retention := 1825
	attachmentMax := 20
	custom := SyncPolicy{
		AccountID:       account.ID,
		LabelID:         "Label_1",
		Include:         true,
		Depth:           "full",
		RetentionDays:   &retention,
		AttachmentRule:  "all",
		AttachmentMaxMB: &attachmentMax,
		UpdatedAt:       time.Date(2026, 4, 24, 13, 0, 0, 0, time.UTC),
	}
	if err := policies.Upsert(ctx, custom); err != nil {
		t.Fatalf("upsert custom policy: %v", err)
	}

	effective, err := policies.EffectiveForLabel(ctx, account.ID, "Label_1")
	if err != nil {
		t.Fatalf("get exact policy: %v", err)
	}
	if effective.LabelID != "Label_1" || effective.Depth != "full" || valueOrZero(effective.RetentionDays) != 1825 {
		t.Fatalf("effective policy = %#v, want custom policy", effective)
	}

	if err := policies.Delete(ctx, account.ID, "Label_1"); err != nil {
		t.Fatalf("delete custom policy: %v", err)
	}
	if _, err := policies.Get(ctx, account.ID, "Label_1"); !errors.Is(err, ErrSyncPolicyNotFound) {
		t.Fatalf("get deleted policy error = %v, want ErrSyncPolicyNotFound", err)
	}
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func labelIDs(labels []Label) []string {
	ids := make([]string, 0, len(labels))
	for _, label := range labels {
		ids = append(ids, label.ID)
	}
	return ids
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
