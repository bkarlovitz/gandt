package cache

import (
	"context"
	"testing"
)

func TestMigrateAppliesSchemaV1(t *testing.T) {
	ctx := context.Background()
	db, err := OpenPath(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var version int
	if err := db.GetContext(ctx, &version, "SELECT version FROM schema_version"); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
	}

	for _, name := range expectedTables() {
		assertSQLiteObjectExists(t, db, "table", name)
	}
	for _, name := range expectedTriggers() {
		assertSQLiteObjectExists(t, db, "trigger", name)
	}
	for _, name := range expectedIndexes() {
		assertSQLiteObjectExists(t, db, "index", name)
	}
}

func TestMigrateAppliesSchemaV1Once(t *testing.T) {
	ctx := context.Background()
	db, err := OpenPath(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer db.Close()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	if err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM schema_version WHERE version = 1"); err != nil {
		t.Fatalf("count schema versions: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema version rows = %d, want 1", count)
	}
}

func assertSQLiteObjectExists(t *testing.T, db queryer, typ, name string) {
	t.Helper()

	var count int
	if err := db.GetContext(context.Background(), &count, "SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?", typ, name); err != nil {
		t.Fatalf("inspect sqlite object %s %s: %v", typ, name, err)
	}
	if count != 1 {
		t.Fatalf("missing sqlite %s %s", typ, name)
	}
}

type queryer interface {
	GetContext(context.Context, any, string, ...any) error
}

func expectedTables() []string {
	return []string{
		"schema_version",
		"accounts",
		"labels",
		"threads",
		"messages",
		"message_labels",
		"attachments",
		"outbox",
		"sync_policies",
		"cache_exclusions",
		"message_annotations",
		"messages_fts",
	}
}

func expectedTriggers() []string {
	return []string{
		"messages_fts_ai",
		"messages_fts_au",
		"messages_fts_ad",
	}
}

func expectedIndexes() []string {
	return []string{
		"sqlite_autoindex_accounts_1",
		"sqlite_autoindex_accounts_2",
		"sqlite_autoindex_labels_1",
		"sqlite_autoindex_threads_1",
		"sqlite_autoindex_messages_1",
		"sqlite_autoindex_message_labels_1",
		"sqlite_autoindex_attachments_1",
		"sqlite_autoindex_outbox_1",
		"sqlite_autoindex_sync_policies_1",
		"sqlite_autoindex_cache_exclusions_1",
		"sqlite_autoindex_message_annotations_1",
		"idx_accounts_email",
		"idx_labels_type",
		"idx_threads_date",
		"idx_messages_thread",
		"idx_messages_date",
		"idx_messages_cached",
		"idx_msglabels_label",
		"idx_attachments_message",
		"idx_outbox_account",
		"idx_sync_policies_label",
		"idx_exclusions_match",
		"idx_annot_namespace",
	}
}
