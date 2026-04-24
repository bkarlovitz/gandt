package ui

import (
	"path/filepath"
	"testing"

	"github.com/bkarlovitz/gandt/internal/config"
)

func TestAttachmentDownloadTargetUsesConfiguredPathAndCachePath(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Downloads = filepath.Join(root, "downloads")
	paths := config.Paths{AttachmentDir: filepath.Join(root, "data", "attachments")}

	target := AttachmentDownloadTargetFor(paths, cfg, "acct/1", "msg/1", "../invoice.pdf")
	if target.Filename != ".._invoice.pdf" {
		t.Fatalf("filename = %q", target.Filename)
	}
	if target.DownloadPath != filepath.Join(root, "downloads", ".._invoice.pdf") {
		t.Fatalf("download path = %q", target.DownloadPath)
	}
	if target.CachePath != filepath.Join(root, "data", "attachments", "acct_1", "msg_1", ".._invoice.pdf") {
		t.Fatalf("cache path = %q", target.CachePath)
	}
}
