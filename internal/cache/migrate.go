package cache

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

const CurrentSchemaVersion = 1

//go:embed schema.sql
var schemaFiles embed.FS

func schemaSQL() (string, error) {
	body, err := schemaFiles.ReadFile("schema.sql")
	if err != nil {
		return "", fmt.Errorf("read cache schema: %w", err)
	}
	return string(body), nil
}

// Migrate applies the v1 public cache schema exactly once.
func Migrate(ctx context.Context, db *sqlx.DB) error {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}

	switch {
	case version == CurrentSchemaVersion:
		return nil
	case version > CurrentSchemaVersion:
		return fmt.Errorf("cache schema version %d is newer than supported version %d", version, CurrentSchemaVersion)
	case version != 0:
		return fmt.Errorf("unsupported cache schema version %d", version)
	}

	schema, err := schemaSQL()
	if err != nil {
		return err
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cache migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply cache schema v1: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version (version, applied_at) VALUES (?, ?)", CurrentSchemaVersion, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cache schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cache migration: %w", err)
	}

	return nil
}

func schemaVersion(ctx context.Context, db *sqlx.DB) (int, error) {
	var exists int
	if err := db.GetContext(ctx, &exists, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_version'"); err != nil {
		return 0, fmt.Errorf("inspect cache schema version table: %w", err)
	}
	if exists == 0 {
		return 0, nil
	}

	var version int
	err := db.GetContext(ctx, &version, "SELECT version FROM schema_version ORDER BY version DESC LIMIT 1")
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cache schema version: %w", err)
	}
	return version, nil
}
