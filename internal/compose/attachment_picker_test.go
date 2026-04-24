package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAttachmentFromPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	attachment, err := AddAttachmentFromPath(path, 10)
	if err != nil {
		t.Fatalf("add attachment: %v", err)
	}
	if attachment.Filename != "note.txt" || attachment.SizeBytes != 5 || string(attachment.Data) != "hello" || !strings.HasPrefix(attachment.MimeType, "text/plain") {
		t.Fatalf("attachment = %#v", attachment)
	}
}

func TestAddAttachmentFromPathValidation(t *testing.T) {
	if _, err := AddAttachmentFromPath("", 0); err == nil {
		t.Fatal("expected missing path error")
	}
	path := filepath.Join(t.TempDir(), "large.bin")
	if err := os.WriteFile(path, []byte("large"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	if _, err := AddAttachmentFromPath(path, 2); err == nil {
		t.Fatal("expected size validation error")
	}
	if _, err := AddAttachmentFromPath(filepath.Dir(path), 0); err == nil {
		t.Fatal("expected directory validation error")
	}
}
