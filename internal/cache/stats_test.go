package cache

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestCacheStatsServiceSummary(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	work := seedStatsAccount(t, db, "work@example.com")
	personal := seedStatsAccount(t, db, "me@example.com")
	if err := seedStatsMessages(ctx, db, work.ID, personal.ID, now); err != nil {
		t.Fatalf("seed stats messages: %v", err)
	}

	stats, err := NewCacheStatsService(db).Summary(ctx, now)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if !stats.GeneratedAt.Equal(now) {
		t.Fatalf("generated at = %v, want %v", stats.GeneratedAt, now)
	}
	if stats.Total.SQLiteBytes <= 0 {
		t.Fatalf("sqlite bytes = %d, want positive database footprint", stats.Total.SQLiteBytes)
	}
	if stats.Total.MessageCount != 3 || stats.Total.BodyCount != 2 {
		t.Fatalf("total counts = messages %d bodies %d, want 3 and 2", stats.Total.MessageCount, stats.Total.BodyCount)
	}
	if stats.Total.MessageBytes != 600 || stats.Total.AttachmentBytes != 40 || stats.Total.FTSBytes <= 0 {
		t.Fatalf("total bytes = %#v, want message bytes 600, attachment bytes 40, positive FTS", stats.Total)
	}

	workStats := accountStatsByEmail(t, stats.Accounts, "work@example.com")
	if workStats.MessageCount != 2 || workStats.BodyCount != 1 || workStats.AttachmentCount != 1 || workStats.AttachmentBytes != 40 {
		t.Fatalf("work stats = %#v, want two messages, one body, one attachment", workStats)
	}
	inboxStats := labelStats(t, stats.Labels, work.ID, "INBOX")
	if inboxStats.MessageCount != 1 || inboxStats.BodyCount != 1 || inboxStats.AttachmentBytes != 40 {
		t.Fatalf("inbox stats = %#v, want one cached body with attachment", inboxStats)
	}
	receiptsStats := labelStats(t, stats.Labels, work.ID, "Receipts")
	if receiptsStats.MessageCount != 1 || receiptsStats.BodyCount != 0 {
		t.Fatalf("receipts stats = %#v, want metadata-only old message", receiptsStats)
	}

	recent := ageStats(t, stats.Ages, "0-7d")
	if recent.MessageCount != 2 || recent.BodyCount != 2 || recent.AttachmentBytes != 40 {
		t.Fatalf("recent age stats = %#v, want two cached recent messages and attachment bytes", recent)
	}
	older := ageStats(t, stats.Ages, "31-90d")
	if older.MessageCount != 1 || older.BodyCount != 0 {
		t.Fatalf("older age stats = %#v, want one metadata message", older)
	}

	if stats.Attachments.AttachmentCount != 1 || stats.Attachments.CachedFileCount != 1 || stats.Attachments.LocalBytes != 40 {
		t.Fatalf("attachment stats = %#v, want one cached local attachment", stats.Attachments)
	}
	if rowCount(stats.Rows, "messages") != 3 || rowCount(stats.Rows, "messages_fts") != 3 {
		t.Fatalf("row counts = %#v, want messages and FTS row counts", stats.Rows)
	}
	if stats.FTS.RowCount != 3 || stats.FTS.Bytes <= 0 {
		t.Fatalf("FTS stats = %#v, want three indexed rows", stats.FTS)
	}
}

func seedStatsAccount(t *testing.T, db *sqlx.DB, email string) Account {
	t.Helper()

	account, err := NewAccountRepository(db).Create(context.Background(), CreateAccountParams{Email: email})
	if err != nil {
		t.Fatalf("create account %s: %v", email, err)
	}
	labels := NewLabelRepository(db)
	for _, label := range []Label{
		{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"},
		{AccountID: account.ID, ID: "Receipts", Name: "Receipts", Type: "user"},
	} {
		if err := labels.Upsert(context.Background(), label); err != nil {
			t.Fatalf("upsert label %s: %v", label.ID, err)
		}
	}
	return account
}

func seedStatsMessages(ctx context.Context, db *sqlx.DB, workID string, personalID string, now time.Time) error {
	threads := NewThreadRepository(db)
	messages := NewMessageRepository(db)
	messageLabels := NewMessageLabelRepository(db)
	attachments := NewAttachmentRepository(db)

	recentBody := "recent cached body"
	personalBody := "personal cached body"
	oldDate := now.AddDate(0, 0, -45)
	fixtures := []struct {
		accountID string
		id        string
		threadID  string
		labelID   string
		from      string
		subject   string
		size      int
		body      *string
		when      time.Time
	}{
		{accountID: workID, id: "work-recent", threadID: "thread-work-recent", labelID: "INBOX", from: "sender@example.com", subject: "Recent", size: 100, body: &recentBody, when: now.Add(-2 * time.Hour)},
		{accountID: workID, id: "work-old", threadID: "thread-work-old", labelID: "Receipts", from: "receipts@example.com", subject: "Receipt", size: 200, when: oldDate},
		{accountID: personalID, id: "personal-recent", threadID: "thread-personal-recent", labelID: "INBOX", from: "friend@example.com", subject: "Hello", size: 300, body: &personalBody, when: now.Add(-24 * time.Hour)},
	}
	for _, fixture := range fixtures {
		if err := threads.Upsert(ctx, Thread{AccountID: fixture.accountID, ID: fixture.threadID, LastMessageDate: &fixture.when}); err != nil {
			return err
		}
		message := Message{
			AccountID:    fixture.accountID,
			ID:           fixture.id,
			ThreadID:     fixture.threadID,
			FromAddr:     fixture.from,
			ToAddrs:      []string{"me@example.com"},
			Subject:      fixture.subject,
			SizeBytes:    fixture.size,
			BodyPlain:    fixture.body,
			InternalDate: &fixture.when,
		}
		if fixture.body != nil {
			message.CachedAt = &fixture.when
		}
		if err := messages.Upsert(ctx, message); err != nil {
			return err
		}
		if err := messageLabels.Upsert(ctx, MessageLabel{AccountID: fixture.accountID, MessageID: fixture.id, LabelID: fixture.labelID}); err != nil {
			return err
		}
	}
	return attachments.Upsert(ctx, Attachment{
		AccountID:    workID,
		MessageID:    "work-recent",
		PartID:       "1",
		Filename:     "invoice.pdf",
		MimeType:     "application/pdf",
		SizeBytes:    40,
		AttachmentID: "att-1",
		LocalPath:    "attachments/invoice.pdf",
	})
}

func accountStatsByEmail(t *testing.T, rows []AccountCacheStats, email string) AccountCacheStats {
	t.Helper()
	for _, row := range rows {
		if row.Email == email {
			return row
		}
	}
	t.Fatalf("missing account stats for %s: %#v", email, rows)
	return AccountCacheStats{}
}

func labelStats(t *testing.T, rows []LabelCacheStats, accountID string, labelID string) LabelCacheStats {
	t.Helper()
	for _, row := range rows {
		if row.AccountID == accountID && row.LabelID == labelID {
			return row
		}
	}
	t.Fatalf("missing label stats for %s/%s: %#v", accountID, labelID, rows)
	return LabelCacheStats{}
}

func ageStats(t *testing.T, rows []AgeCacheStats, bucket string) AgeCacheStats {
	t.Helper()
	for _, row := range rows {
		if row.Bucket == bucket {
			return row
		}
	}
	t.Fatalf("missing age stats for %s: %#v", bucket, rows)
	return AgeCacheStats{}
}

func rowCount(rows []TableRowCount, table string) int {
	for _, row := range rows {
		if row.Table == table {
			return row.Rows
		}
	}
	return 0
}
