package gmail

import (
	"context"
	"errors"

	gmailapi "google.golang.org/api/gmail/v1"
)

type MessageModifyRequest struct {
	IDs            []string
	AddLabelIDs    []string
	RemoveLabelIDs []string
}

type ThreadModifyRequest struct {
	ThreadID       string
	AddLabelIDs    []string
	RemoveLabelIDs []string
}

type LabelCreateRequest struct {
	Name                  string
	LabelListVisibility   string
	MessageListVisibility string
	ColorBG               string
	ColorFG               string
}

func (c *Client) BatchModifyMessages(ctx context.Context, request MessageModifyRequest) error {
	if len(request.IDs) == 0 {
		return errors.New("message ids are required")
	}
	body := &gmailapi.BatchModifyMessagesRequest{
		Ids:            append([]string{}, request.IDs...),
		AddLabelIds:    append([]string{}, request.AddLabelIDs...),
		RemoveLabelIds: append([]string{}, request.RemoveLabelIDs...),
	}
	if err := c.withRetry(ctx, "batch modify gmail messages", func() error {
		return c.service.Users.Messages.BatchModify("me", body).Context(ctx).Do()
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) TrashMessage(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("message id is required")
	}
	if err := c.withRetry(ctx, "trash gmail message", func() error {
		_, err := c.service.Users.Messages.Trash("me", id).Context(ctx).Do()
		return err
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) UntrashMessage(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("message id is required")
	}
	if err := c.withRetry(ctx, "untrash gmail message", func() error {
		_, err := c.service.Users.Messages.Untrash("me", id).Context(ctx).Do()
		return err
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) ModifyThread(ctx context.Context, request ThreadModifyRequest) error {
	if request.ThreadID == "" {
		return errors.New("thread id is required")
	}
	body := &gmailapi.ModifyThreadRequest{
		AddLabelIds:    append([]string{}, request.AddLabelIDs...),
		RemoveLabelIds: append([]string{}, request.RemoveLabelIDs...),
	}
	if err := c.withRetry(ctx, "modify gmail thread", func() error {
		_, err := c.service.Users.Threads.Modify("me", request.ThreadID, body).Context(ctx).Do()
		return err
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) CreateLabel(ctx context.Context, request LabelCreateRequest) (Label, error) {
	if request.Name == "" {
		return Label{}, errors.New("label name is required")
	}
	body := &gmailapi.Label{
		Name:                  request.Name,
		LabelListVisibility:   request.LabelListVisibility,
		MessageListVisibility: request.MessageListVisibility,
	}
	if request.ColorBG != "" || request.ColorFG != "" {
		body.Color = &gmailapi.LabelColor{
			BackgroundColor: request.ColorBG,
			TextColor:       request.ColorFG,
		}
	}
	var label *gmailapi.Label
	if err := c.withRetry(ctx, "create gmail label", func() error {
		var err error
		label, err = c.service.Users.Labels.Create("me", body).Context(ctx).Do()
		return err
	}); err != nil {
		return Label{}, err
	}
	return convertLabel(label), nil
}

func (c *Client) DeleteLabel(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("label id is required")
	}
	if err := c.withRetry(ctx, "delete gmail label", func() error {
		return c.service.Users.Labels.Delete("me", id).Context(ctx).Do()
	}); err != nil {
		return err
	}
	return nil
}
