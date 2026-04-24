package ui

import (
	"context"

	"github.com/bkarlovitz/gandt/internal/compose"
)

type AttachmentOpenState struct {
	Path   string
	Status string
	Err    string
}

func OpenAttachmentForUI(ctx context.Context, request compose.AttachmentOpenRequest) AttachmentOpenState {
	path, err := compose.OpenAttachment(ctx, request)
	if err != nil {
		return AttachmentOpenState{Path: path, Status: "error", Err: err.Error()}
	}
	return AttachmentOpenState{Path: path, Status: "opened"}
}
