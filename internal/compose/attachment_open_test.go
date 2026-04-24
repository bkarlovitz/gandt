package compose

import (
	"context"
	"errors"
	"testing"
)

func TestOpenAttachmentUsesCachedPath(t *testing.T) {
	opened := ""
	path, err := OpenAttachment(context.Background(), AttachmentOpenRequest{
		LocalPath: "/tmp/one.pdf",
		Stat:      func(string) error { return nil },
		Open: func(_ context.Context, path string) error {
			opened = path
			return nil
		},
	})
	if err != nil {
		t.Fatalf("open attachment: %v", err)
	}
	if path != "/tmp/one.pdf" || opened != "/tmp/one.pdf" {
		t.Fatalf("path=%q opened=%q", path, opened)
	}
}

func TestOpenAttachmentLazyFetchesWhenMissing(t *testing.T) {
	opened := ""
	fetched := false
	path, err := OpenAttachment(context.Background(), AttachmentOpenRequest{
		LocalPath: "/tmp/missing.pdf",
		Stat:      func(string) error { return errors.New("missing") },
		Fetch: func(context.Context) (string, error) {
			fetched = true
			return "/tmp/fetched.pdf", nil
		},
		Open: func(_ context.Context, path string) error {
			opened = path
			return nil
		},
	})
	if err != nil {
		t.Fatalf("open attachment: %v", err)
	}
	if !fetched || path != "/tmp/fetched.pdf" || opened != "/tmp/fetched.pdf" {
		t.Fatalf("fetched=%v path=%q opened=%q", fetched, path, opened)
	}
}

func TestOpenAttachmentMissingOpener(t *testing.T) {
	_, err := OpenAttachmentOnPlatform(context.Background(), AttachmentOpenRequest{
		LocalPath: "/tmp/one.pdf",
		Stat:      func(string) error { return nil },
		Open:      nil,
	}, "plan9")
	if !errors.Is(err, ErrAttachmentOpenerUnavailable) {
		t.Fatalf("error = %v, want opener unavailable", err)
	}

	if SystemPathOpener("plan9") != nil {
		t.Fatal("unsupported platform should not have opener")
	}
}
