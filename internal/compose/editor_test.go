package compose

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEditBodyExternalSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake editor is unix-only")
	}
	dir := t.TempDir()
	editor := fakeEditor(t, dir, "printf '\\nedited' >> \"$1\"\n")

	result, err := EditBody(context.Background(), BodyEditorRequest{
		InitialText: "prefill\n> quoted",
		Mode:        BodyModeExternal,
		Editor:      editor,
		TempDir:     dir,
	})
	if err != nil {
		t.Fatalf("edit body: %v", err)
	}
	if !result.UsedExternal || result.Mode != BodyModeExternal {
		t.Fatalf("result = %#v, want external editor used", result)
	}
	if result.Body != "prefill\n> quoted\nedited" {
		t.Fatalf("body = %q, want prefill plus editor changes", result.Body)
	}
	if _, err := os.Stat(result.TempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tempfile still exists or stat failed: %v", err)
	}
}

func TestEditBodyFallbacksToInline(t *testing.T) {
	result, err := EditBody(context.Background(), BodyEditorRequest{
		InitialText: "draft",
		Mode:        BodyModeExternal,
		Editor:      "",
	})
	if err != nil {
		t.Fatalf("unset editor fallback: %v", err)
	}
	if result.UsedExternal || result.Mode != BodyModeInline || result.Body != "draft" {
		t.Fatalf("unset editor result = %#v, want inline draft", result)
	}

	result, err = EditBody(context.Background(), BodyEditorRequest{
		InitialText: "forced inline",
		Mode:        BodyModeInline,
		Editor:      "ignored",
	})
	if err != nil {
		t.Fatalf("forced inline: %v", err)
	}
	if result.UsedExternal || result.Mode != BodyModeInline || result.Body != "forced inline" {
		t.Fatalf("forced inline result = %#v", result)
	}
}

func TestEditBodyReturnsTextAndCleansUpOnEditorFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake editor is unix-only")
	}
	dir := t.TempDir()
	editor := fakeEditor(t, dir, "printf '\\nkept' >> \"$1\"\nexit 7\n")

	result, err := EditBody(context.Background(), BodyEditorRequest{
		InitialText: "draft",
		Mode:        BodyModeExternal,
		Editor:      editor,
		TempDir:     dir,
	})
	if !errors.Is(err, ErrEditorFailed) {
		t.Fatalf("error = %v, want ErrEditorFailed", err)
	}
	if result.Body != "draft\nkept" {
		t.Fatalf("body = %q, want text preserved despite failure", result.Body)
	}
	if _, err := os.Stat(result.TempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tempfile still exists or stat failed: %v", err)
	}
}

func fakeEditor(t *testing.T, dir string, script string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-editor.sh")
	body := "#!/bin/sh\nset -eu\n" + strings.TrimPrefix(script, "\n")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}
	return path
}
