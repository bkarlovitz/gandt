package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bkarlovitz/gandt/internal/config"
)

type CacheWipeResult struct {
	DatabaseFilesRemoved   int
	AttachmentFilesRemoved int
	AttachmentDeleteErrors []string
}

func Wipe(ctx context.Context, paths config.Paths) (CacheWipeResult, error) {
	dbPath, err := Path(paths)
	if err != nil {
		return CacheWipeResult{}, err
	}
	result := CacheWipeResult{}

	attachmentPaths, err := wipeAttachmentPaths(ctx, dbPath)
	if err != nil {
		return CacheWipeResult{}, err
	}
	for _, path := range attachmentPaths {
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			result.AttachmentDeleteErrors = append(result.AttachmentDeleteErrors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		result.AttachmentFilesRemoved++
	}
	if err := os.RemoveAll(filepath.Join(paths.DataDir, "attachments")); err != nil {
		result.AttachmentDeleteErrors = append(result.AttachmentDeleteErrors, fmt.Sprintf("attachments directory: %v", err))
	}

	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		removed, err := removeFileIfExists(path)
		if err != nil {
			return CacheWipeResult{}, err
		}
		if removed {
			result.DatabaseFilesRemoved++
		}
	}
	return result, nil
}

func wipeAttachmentPaths(ctx context.Context, dbPath string) ([]string, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	db, err := OpenPath(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var exists int
	if err := db.GetContext(ctx, &exists, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'attachments'"); err != nil {
		return nil, fmt.Errorf("inspect attachments table: %w", err)
	}
	if exists == 0 {
		return nil, nil
	}
	paths := []string{}
	if err := db.SelectContext(ctx, &paths, "SELECT local_path FROM attachments WHERE local_path IS NOT NULL AND local_path != ''"); err != nil {
		return nil, fmt.Errorf("list attachment paths: %w", err)
	}
	return paths, nil
}

func removeFileIfExists(path string) (bool, error) {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
