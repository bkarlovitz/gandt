package gmail

import (
	"context"
	"encoding/base64"
	"errors"

	gmailapi "google.golang.org/api/gmail/v1"
)

type DraftRef struct {
	ID      string
	Message MessageRef
}

type DraftsPage struct {
	Drafts        []DraftRef
	NextPageToken string
	ResultSize    int
}

type ListDraftsOptions struct {
	Query      string
	PageToken  string
	MaxResults int64
}

func (c *Client) ListDrafts(ctx context.Context, opts ListDraftsOptions) (DraftsPage, error) {
	var response *gmailapi.ListDraftsResponse
	if err := c.withRetry(ctx, "list gmail drafts", func() error {
		call := c.service.Users.Drafts.List("me").Context(ctx)
		if opts.Query != "" {
			call.Q(opts.Query)
		}
		if opts.PageToken != "" {
			call.PageToken(opts.PageToken)
		}
		if opts.MaxResults > 0 {
			call.MaxResults(opts.MaxResults)
		}
		var err error
		response, err = call.Do()
		return err
	}); err != nil {
		return DraftsPage{}, err
	}
	return convertDraftsPage(response), nil
}

func (c *Client) GetDraft(ctx context.Context, id string, format MessageFormat) (DraftRef, Message, error) {
	if id == "" {
		return DraftRef{}, Message{}, errors.New("draft id is required")
	}
	if format == "" {
		format = MessageFormatFull
	}
	var response *gmailapi.Draft
	if err := c.withRetry(ctx, "get gmail draft", func() error {
		var err error
		response, err = c.service.Users.Drafts.Get("me", id).Format(string(format)).Context(ctx).Do()
		return err
	}); err != nil {
		return DraftRef{}, Message{}, err
	}
	return convertDraftRef(response), convertMessage(response.Message), nil
}

func (c *Client) CreateDraft(ctx context.Context, raw []byte) (DraftRef, error) {
	var response *gmailapi.Draft
	if err := c.withRetry(ctx, "create gmail draft", func() error {
		var err error
		response, err = c.service.Users.Drafts.Create("me", rawDraft(raw)).Context(ctx).Do()
		return err
	}); err != nil {
		return DraftRef{}, err
	}
	return convertDraftRef(response), nil
}

func (c *Client) UpdateDraft(ctx context.Context, id string, raw []byte) (DraftRef, error) {
	if id == "" {
		return DraftRef{}, errors.New("draft id is required")
	}
	var response *gmailapi.Draft
	if err := c.withRetry(ctx, "update gmail draft", func() error {
		var err error
		response, err = c.service.Users.Drafts.Update("me", id, rawDraft(raw)).Context(ctx).Do()
		return err
	}); err != nil {
		return DraftRef{}, err
	}
	return convertDraftRef(response), nil
}

func (c *Client) DeleteDraft(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("draft id is required")
	}
	return c.withRetry(ctx, "delete gmail draft", func() error {
		return c.service.Users.Drafts.Delete("me", id).Context(ctx).Do()
	})
}

func (c *Client) SendDraft(ctx context.Context, id string) (MessageRef, error) {
	if id == "" {
		return MessageRef{}, errors.New("draft id is required")
	}
	var response *gmailapi.Message
	if err := c.withRetry(ctx, "send gmail draft", func() error {
		var err error
		response, err = c.service.Users.Drafts.Send("me", &gmailapi.Draft{Id: id}).Context(ctx).Do()
		return err
	}); err != nil {
		return MessageRef{}, err
	}
	message := convertMessage(response)
	return MessageRef{ID: message.ID, ThreadID: message.ThreadID}, nil
}

func rawDraft(raw []byte) *gmailapi.Draft {
	return &gmailapi.Draft{Message: &gmailapi.Message{Raw: base64.RawURLEncoding.EncodeToString(raw)}}
}

func convertDraftsPage(response *gmailapi.ListDraftsResponse) DraftsPage {
	if response == nil {
		return DraftsPage{}
	}
	drafts := make([]DraftRef, 0, len(response.Drafts))
	for _, draft := range response.Drafts {
		drafts = append(drafts, convertDraftRef(draft))
	}
	return DraftsPage{
		Drafts:        drafts,
		NextPageToken: response.NextPageToken,
		ResultSize:    int(response.ResultSizeEstimate),
	}
}

func convertDraftRef(draft *gmailapi.Draft) DraftRef {
	if draft == nil {
		return DraftRef{}
	}
	ref := DraftRef{ID: draft.Id}
	if draft.Message != nil {
		ref.Message = MessageRef{ID: draft.Message.Id, ThreadID: draft.Message.ThreadId}
	}
	return ref
}
