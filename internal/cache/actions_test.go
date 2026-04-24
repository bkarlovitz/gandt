package cache

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestOptimisticActionRepositoryAppliesAndRevertsActionFamilies(t *testing.T) {
	tests := []struct {
		name   string
		action OptimisticAction
		start  []string
		want   []string
	}{
		{name: "archive", action: OptimisticAction{Kind: OptimisticArchive}, start: []string{"INBOX", "UNREAD"}, want: []string{"UNREAD"}},
		{name: "trash", action: OptimisticAction{Kind: OptimisticTrash}, start: []string{"INBOX"}, want: []string{"TRASH"}},
		{name: "spam", action: OptimisticAction{Kind: OptimisticSpam}, start: []string{"INBOX"}, want: []string{"SPAM"}},
		{name: "star", action: OptimisticAction{Kind: OptimisticToggleStar, Add: true}, start: []string{"INBOX"}, want: []string{"INBOX", "STARRED"}},
		{name: "unstar", action: OptimisticAction{Kind: OptimisticToggleStar, Add: false}, start: []string{"INBOX", "STARRED"}, want: []string{"INBOX"}},
		{name: "unread", action: OptimisticAction{Kind: OptimisticToggleUnread, Add: true}, start: []string{"INBOX"}, want: []string{"INBOX", "UNREAD"}},
		{name: "read", action: OptimisticAction{Kind: OptimisticToggleUnread, Add: false}, start: []string{"INBOX", "UNREAD"}, want: []string{"INBOX"}},
		{name: "label add", action: OptimisticAction{Kind: OptimisticLabelAdd, LabelID: "Label_1"}, start: []string{"INBOX"}, want: []string{"INBOX", "Label_1"}},
		{name: "label remove", action: OptimisticAction{Kind: OptimisticLabelRemove, LabelID: "Label_1"}, start: []string{"INBOX", "Label_1"}, want: []string{"INBOX"}},
		{name: "mute", action: OptimisticAction{Kind: OptimisticMute}, start: []string{"INBOX"}, want: []string{"INBOX", "MUTED"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := migratedTestDB(t)
			account := seedActionAccount(t, db)
			seedActionMessage(t, db, account.ID, tt.start)
			tt.action.AccountID = account.ID
			tt.action.MessageID = "message-1"

			repo := NewOptimisticActionRepository(db)
			snapshot, err := repo.Apply(ctx, tt.action)
			if err != nil {
				t.Fatalf("apply action: %v", err)
			}
			assertMessageLabels(t, db, account.ID, "message-1", tt.want)
			if err := repo.Revert(ctx, snapshot); err != nil {
				t.Fatalf("revert action: %v", err)
			}
			assertMessageLabels(t, db, account.ID, "message-1", tt.start)
		})
	}
}

func seedActionAccount(t *testing.T, db *sqlx.DB) Account {
	t.Helper()

	account, err := NewAccountRepository(db).Create(context.Background(), CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	labels := []Label{
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "TRASH", Name: "Trash", Type: "system"},
		{AccountID: account.ID, ID: "SPAM", Name: "Spam", Type: "system"},
		{AccountID: account.ID, ID: "STARRED", Name: "Starred", Type: "system"},
		{AccountID: account.ID, ID: "UNREAD", Name: "Unread", Type: "system"},
		{AccountID: account.ID, ID: "MUTED", Name: "Muted", Type: "system"},
		{AccountID: account.ID, ID: "Label_1", Name: "Label 1", Type: "user"},
	}
	for _, label := range labels {
		if err := NewLabelRepository(db).Upsert(context.Background(), label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}
	return account
}

func seedActionMessage(t *testing.T, db *sqlx.DB, accountID string, labels []string) {
	t.Helper()

	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if err := NewThreadRepository(db).Upsert(context.Background(), Thread{AccountID: accountID, ID: "thread-1", LastMessageDate: &now}); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	if err := NewMessageRepository(db).Upsert(context.Background(), Message{AccountID: accountID, ID: "message-1", ThreadID: "thread-1", InternalDate: &now}); err != nil {
		t.Fatalf("upsert message: %v", err)
	}
	if err := NewMessageLabelRepository(db).ReplaceForMessage(context.Background(), accountID, "message-1", labels); err != nil {
		t.Fatalf("replace labels: %v", err)
	}
}

func assertMessageLabels(t *testing.T, db *sqlx.DB, accountID string, messageID string, want []string) {
	t.Helper()

	labels, err := NewMessageLabelRepository(db).ListForMessage(context.Background(), accountID, messageID)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	got := make([]string, 0, len(labels))
	for _, label := range labels {
		got = append(got, label.LabelID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
}
