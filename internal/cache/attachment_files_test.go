package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAttachmentBytesCachesAndDownloads(t *testing.T) {
	ctx := context.Background()
	db := openMigratedCache(t)
	defer db.Close()
	accountID := "acct-1"
	seedAccount(t, db, accountID, "me@example.com")
	threads := NewThreadRepository(db)
	messages := NewMessageRepository(db)
	attachments := NewAttachmentRepository(db)
	if err := threads.Upsert(ctx, Thread{AccountID: accountID, ID: "thread-1"}); err != nil {
		t.Fatalf("thread: %v", err)
	}
	if err := messages.Upsert(ctx, Message{AccountID: accountID, ID: "msg-1", ThreadID: "thread-1", CachedAt: timePtr(time.Now().UTC())}); err != nil {
		t.Fatalf("message: %v", err)
	}

	root := t.TempDir()
	result, err := SaveAttachmentBytes(ctx, attachments, Attachment{
		AccountID:    accountID,
		MessageID:    "msg-1",
		PartID:       "1",
		Filename:     "../report.pdf",
		MimeType:     "application/pdf",
		SizeBytes:    4,
		AttachmentID: "att-1",
	}, []byte("data"), filepath.Join(root, "attachments"), filepath.Join(root, "downloads"))
	if err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if filepath.Base(result.CachePath) != ".._report.pdf" || filepath.Base(result.DownloadPath) != ".._report.pdf" {
		t.Fatalf("paths = %#v, want safe filename", result)
	}
	cached, err := os.ReadFile(result.CachePath)
	if err != nil || string(cached) != "data" {
		t.Fatalf("cached bytes = %q err=%v", string(cached), err)
	}
	downloaded, err := os.ReadFile(result.DownloadPath)
	if err != nil || string(downloaded) != "data" {
		t.Fatalf("downloaded bytes = %q err=%v", string(downloaded), err)
	}
	row, err := attachments.Get(ctx, accountID, "msg-1", "1")
	if err != nil || row.LocalPath != result.CachePath {
		t.Fatalf("attachment row = %#v err=%v", row, err)
	}
}

func TestSafeFilenameFallback(t *testing.T) {
	if got := SafeFilename(""); got != "attachment" {
		t.Fatalf("empty safe filename = %q", got)
	}
	if got := SafeFilename(`nested\file.txt`); got != "nested_file.txt" {
		t.Fatalf("safe filename = %q", got)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
