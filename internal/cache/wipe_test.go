package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestWipeDeletesCacheAndAttachmentsThenRemigrates(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:  filepath.Join(root, "config"),
		DataDir:    filepath.Join(root, "data"),
		ConfigFile: filepath.Join(root, "config", "config.toml"),
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(paths.ConfigFile, []byte("client credentials stay outside cache\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	db, err := Open(ctx, paths)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	account, err := NewAccountRepository(db).Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := NewLabelRepository(db).Upsert(ctx, Label{AccountID: account.ID, ID: "INBOX", Name: "Inbox", Type: "system"}); err != nil {
		t.Fatalf("upsert label: %v", err)
	}
	when := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if err := NewThreadRepository(db).Upsert(ctx, Thread{AccountID: account.ID, ID: "thread-1", LastMessageDate: &when}); err != nil {
		t.Fatalf("upsert thread: %v", err)
	}
	if err := NewMessageRepository(db).Upsert(ctx, Message{AccountID: account.ID, ID: "message-1", ThreadID: "thread-1", InternalDate: &when}); err != nil {
		t.Fatalf("upsert message: %v", err)
	}
	attachmentPath := filepath.Join(paths.DataDir, "attachments", "one.pdf")
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o700); err != nil {
		t.Fatalf("create attachments dir: %v", err)
	}
	if err := os.WriteFile(attachmentPath, []byte("attachment"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	if err := NewAttachmentRepository(db).Upsert(ctx, Attachment{AccountID: account.ID, MessageID: "message-1", PartID: "1", Filename: "one.pdf", LocalPath: attachmentPath}); err != nil {
		t.Fatalf("upsert attachment: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	result, err := Wipe(ctx, paths)
	if err != nil {
		t.Fatalf("wipe cache: %v", err)
	}
	if result.DatabaseFilesRemoved == 0 || result.AttachmentFilesRemoved != 1 || len(result.AttachmentDeleteErrors) != 0 {
		t.Fatalf("wipe result = %#v, want database and one attachment removed", result)
	}
	dbPath, err := Path(paths)
	if err != nil {
		t.Fatalf("cache path: %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("cache db stat = %v, want removed database", err)
	}
	if _, err := os.Stat(attachmentPath); !os.IsNotExist(err) {
		t.Fatalf("attachment stat = %v, want removed attachment", err)
	}
	if _, err := os.Stat(paths.ConfigFile); err != nil {
		t.Fatalf("config file was removed: %v", err)
	}

	reopened, err := Open(ctx, paths)
	if err != nil {
		t.Fatalf("reopen cache: %v", err)
	}
	defer reopened.Close()
	if err := Migrate(ctx, reopened); err != nil {
		t.Fatalf("remigrate cache: %v", err)
	}
	var version int
	if err := reopened.GetContext(ctx, &version, "SELECT version FROM schema_version ORDER BY version DESC LIMIT 1"); err != nil {
		t.Fatalf("read remigrated schema: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
	}
}
