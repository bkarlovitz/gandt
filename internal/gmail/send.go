package gmail

import (
	"context"
	"encoding/base64"

	gmailapi "google.golang.org/api/gmail/v1"
)

func (c *Client) SendMessage(ctx context.Context, raw []byte) (MessageRef, error) {
	var response *gmailapi.Message
	if err := c.withRetry(ctx, "send gmail message", func() error {
		var err error
		response, err = c.service.Users.Messages.Send("me", &gmailapi.Message{
			Raw: base64.RawURLEncoding.EncodeToString(raw),
		}).Context(ctx).Do()
		return err
	}); err != nil {
		return MessageRef{}, err
	}
	message := convertMessage(response)
	return MessageRef{ID: message.ID, ThreadID: message.ThreadID}, nil
}
