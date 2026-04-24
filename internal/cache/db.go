package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const DatabaseFileName = "cache.db"

// Path returns the SQLite cache location under the configured data directory.
func Path(paths config.Paths) (string, error) {
	if paths.DataDir == "" {
		return "", errors.New("cache data directory is empty")
	}
	return filepath.Join(paths.DataDir, DatabaseFileName), nil
}

// Open opens the public SQLite cache database and applies runtime pragmas.
func Open(ctx context.Context, paths config.Paths) (*sqlx.DB, error) {
	path, err := Path(paths)
	if err != nil {
		return nil, err
	}
	return OpenPath(ctx, path)
}

// OpenPath exists for tests and tools that need to inspect an explicit cache path.
func OpenPath(ctx context.Context, path string) (*sqlx.DB, error) {
	if path == "" {
		return nil, errors.New("cache database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open cache database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := configure(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func configure(ctx context.Context, db *sqlx.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping cache database: %w", err)
	}

	pragmas := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure cache database: %w", err)
		}
	}

	return nil
}
