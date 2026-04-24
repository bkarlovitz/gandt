package compose

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func AddAttachmentFromPath(path string, maxBytes int64) (Attachment, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Attachment{}, errors.New("attachment path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("stat attachment: %w", err)
	}
	if info.IsDir() {
		return Attachment{}, errors.New("attachment path is a directory")
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return Attachment{}, fmt.Errorf("attachment exceeds size limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("read attachment: %w", err)
	}
	mimeType := http.DetectContentType(data)
	attachment := NewAttachment(path, info.Size(), mimeType)
	attachment.Data = data
	return attachment, attachment.ValidateMetadata()
}
