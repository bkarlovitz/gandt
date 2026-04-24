package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AttachmentSaveResult struct {
	CachePath    string
	DownloadPath string
}

func SaveAttachmentBytes(ctx context.Context, repo AttachmentRepository, attachment Attachment, data []byte, attachmentRoot string, downloadsDir string) (AttachmentSaveResult, error) {
	if attachmentRoot == "" {
		return AttachmentSaveResult{}, errors.New("attachment cache root is required")
	}
	if downloadsDir == "" {
		return AttachmentSaveResult{}, errors.New("downloads directory is required")
	}
	filename := SafeFilename(attachment.Filename)
	cachePath := filepath.Join(attachmentRoot, SafeFilename(attachment.AccountID), SafeFilename(attachment.MessageID), filename)
	downloadPath := filepath.Join(downloadsDir, filename)
	for _, path := range []string{filepath.Dir(cachePath), filepath.Dir(downloadPath)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return AttachmentSaveResult{}, fmt.Errorf("create attachment directory: %w", err)
		}
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		return AttachmentSaveResult{}, fmt.Errorf("write cached attachment: %w", err)
	}
	if err := os.WriteFile(downloadPath, data, 0o600); err != nil {
		return AttachmentSaveResult{}, fmt.Errorf("write downloaded attachment: %w", err)
	}
	attachment.LocalPath = cachePath
	if err := repo.Upsert(ctx, attachment); err != nil {
		return AttachmentSaveResult{}, err
	}
	return AttachmentSaveResult{CachePath: cachePath, DownloadPath: downloadPath}, nil
}

func SafeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "attachment"
	}
	return name
}
