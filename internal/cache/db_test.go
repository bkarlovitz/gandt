package cache

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestPathUsesConfiguredDataDir(t *testing.T) {
	root := t.TempDir()
	got, err := Path(config.Paths{DataDir: root})
	if err != nil {
		t.Fatalf("path: %v", err)
	}

	want := filepath.Join(root, DatabaseFileName)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestOpenEnablesWALAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, config.Paths{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.GetContext(ctx, &journalMode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}

	var foreignKeys int
	if err := db.GetContext(ctx, &foreignKeys, "PRAGMA foreign_keys"); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
}

func TestOpenCloseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cache.db")

	db, err := OpenPath(ctx, path)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE ok (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	reopened, err := OpenPath(ctx, path)
	if err != nil {
		t.Fatalf("reopen cache: %v", err)
	}
	defer reopened.Close()

	var name string
	if err := reopened.GetContext(ctx, &name, "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'ok'"); err != nil {
		t.Fatalf("read reopened schema: %v", err)
	}
	if name != "ok" {
		t.Fatalf("table name = %q, want ok", name)
	}
}
