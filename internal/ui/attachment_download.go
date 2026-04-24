package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
)

type AttachmentDownloadTarget struct {
	Filename     string
	DownloadDir  string
	DownloadPath string
	CachePath    string
}

func AttachmentDownloadTargetFor(paths config.Paths, cfg config.Config, accountID string, messageID string, filename string) AttachmentDownloadTarget {
	safe := cache.SafeFilename(filename)
	downloadDir := resolveDownloadDir(cfg.Paths.Downloads)
	return AttachmentDownloadTarget{
		Filename:     safe,
		DownloadDir:  downloadDir,
		DownloadPath: filepath.Join(downloadDir, safe),
		CachePath:    filepath.Join(paths.AttachmentDir, cache.SafeFilename(accountID), cache.SafeFilename(messageID), safe),
	}
}

func resolveDownloadDir(configured string) string {
	if configured == "" {
		configured = os.Getenv("XDG_DOWNLOAD_DIR")
	}
	if configured == "" {
		configured = "~/Downloads"
	}
	if strings.HasPrefix(configured, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(configured, "~/"))
		}
	}
	return configured
}
