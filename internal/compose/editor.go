package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrEditorFailed = errors.New("external editor failed")

type BodyEditorRequest struct {
	InitialText string
	Mode        BodyMode
	Editor      string
	TempDir     string
}

type BodyEditorResult struct {
	Body         string
	Mode         BodyMode
	UsedExternal bool
	TempPath     string
}

func EditBody(ctx context.Context, request BodyEditorRequest) (BodyEditorResult, error) {
	editor := strings.TrimSpace(request.Editor)
	if request.Mode == BodyModeInline || editor == "" {
		return BodyEditorResult{
			Body: request.InitialText,
			Mode: BodyModeInline,
		}, nil
	}

	file, err := os.CreateTemp(request.TempDir, "gandt-compose-*.txt")
	if err != nil {
		return BodyEditorResult{}, fmt.Errorf("create compose tempfile: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(request.InitialText); err != nil {
		file.Close()
		os.Remove(path)
		return BodyEditorResult{}, fmt.Errorf("write compose tempfile: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return BodyEditorResult{}, fmt.Errorf("close compose tempfile: %w", err)
	}

	runErr := runEditor(ctx, editor, path)
	bodyBytes, readErr := os.ReadFile(path)
	removeErr := os.Remove(path)
	body := string(bodyBytes)
	result := BodyEditorResult{
		Body:         body,
		Mode:         BodyModeExternal,
		UsedExternal: true,
		TempPath:     path,
	}
	if readErr != nil {
		return result, fmt.Errorf("read compose tempfile: %w", readErr)
	}
	if removeErr != nil {
		return result, fmt.Errorf("remove compose tempfile: %w", removeErr)
	}
	if runErr != nil {
		return result, fmt.Errorf("%w: %v", ErrEditorFailed, runErr)
	}
	return result, nil
}

func runEditor(ctx context.Context, editor string, path string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", editor+" %1", path)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", editor+" \"$1\"", "gandt-editor", path)
	}
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, message)
	}
	return nil
}
