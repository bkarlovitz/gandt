package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bkarlovitz/gandt/internal/compose"
)

func TestOpenAttachmentForUISuccessAndError(t *testing.T) {
	state := OpenAttachmentForUI(context.Background(), compose.AttachmentOpenRequest{
		LocalPath: "/tmp/one.pdf",
		Stat:      func(string) error { return nil },
		Open:      func(context.Context, string) error { return nil },
	})
	if state.Status != "opened" || state.Path != "/tmp/one.pdf" || state.Err != "" {
		t.Fatalf("state = %#v", state)
	}

	state = OpenAttachmentForUI(context.Background(), compose.AttachmentOpenRequest{
		LocalPath: "",
		Fetch: func(context.Context) (string, error) {
			return "", errors.New("fetch failed")
		},
		Open: func(context.Context, string) error { return nil },
	})
	if state.Status != "error" || !strings.Contains(state.Err, "fetch failed") {
		t.Fatalf("state = %#v", state)
	}
}
