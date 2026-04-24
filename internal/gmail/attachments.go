package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	gmailapi "google.golang.org/api/gmail/v1"
)

func (c *Client) GetAttachment(ctx context.Context, messageID string, attachmentID string) ([]byte, error) {
	if messageID == "" {
		return nil, errors.New("message id is required")
	}
	if attachmentID == "" {
		return nil, errors.New("attachment id is required")
	}
	var body *gmailapi.MessagePartBody
	if err := c.withRetry(ctx, "get gmail attachment", func() error {
		var err error
		body, err = c.service.Users.Messages.Attachments.Get("me", messageID, attachmentID).Context(ctx).Do()
		return err
	}); err != nil {
		return nil, err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(body.Data)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(body.Data)
	}
	if err != nil {
		return nil, fmt.Errorf("decode gmail attachment: %w", err)
	}
	return decoded, nil
}
