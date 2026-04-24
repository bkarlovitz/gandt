package compose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

var ErrAttachmentOpenerUnavailable = errors.New("attachment opener unavailable")

type PathOpener func(context.Context, string) error
type LazyAttachmentFetch func(context.Context) (string, error)

type AttachmentOpenRequest struct {
	LocalPath string
	Open      PathOpener
	Fetch     LazyAttachmentFetch
	Stat      func(string) error
}

func OpenAttachment(ctx context.Context, request AttachmentOpenRequest) (string, error) {
	return OpenAttachmentOnPlatform(ctx, request, runtime.GOOS)
}

func OpenAttachmentOnPlatform(ctx context.Context, request AttachmentOpenRequest, goos string) (string, error) {
	path := request.LocalPath
	stat := request.Stat
	if stat == nil {
		stat = func(path string) error {
			_, err := os.Stat(path)
			return err
		}
	}
	if path == "" || stat(path) != nil {
		if request.Fetch == nil {
			return "", errors.New("attachment is not cached")
		}
		fetched, err := request.Fetch(ctx)
		if err != nil {
			return "", err
		}
		path = fetched
	}
	opener := request.Open
	if opener == nil {
		opener = SystemPathOpener(goos)
	}
	if opener == nil {
		return "", ErrAttachmentOpenerUnavailable
	}
	if err := opener(ctx, path); err != nil {
		return path, fmt.Errorf("open attachment: %w", err)
	}
	return path, nil
}

func SystemPathOpener(goos string) PathOpener {
	switch goos {
	case "darwin":
		return execPathOpener("open")
	case "linux":
		return execPathOpener("xdg-open")
	case "windows":
		return func(ctx context.Context, path string) error {
			return exec.CommandContext(ctx, "cmd", "/C", "start", "", path).Run()
		}
	default:
		return nil
	}
}

func execPathOpener(name string) PathOpener {
	return func(ctx context.Context, path string) error {
		return exec.CommandContext(ctx, name, path).Run()
	}
}
