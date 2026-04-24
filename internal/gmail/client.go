package gmail

import (
	"context"
	"fmt"

	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Profile struct {
	EmailAddress string
	HistoryID    string
}

type Label struct {
	ID      string
	Name    string
	Type    string
	Unread  int
	Total   int
	ColorBG string
	ColorFG string
}

type Client struct {
	service *gmailapi.Service
	retry   RetryPolicy
}

func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	service, err := gmailapi.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Client{service: service, retry: DefaultRetryPolicy()}, nil
}

func (c *Client) Profile(ctx context.Context) (Profile, error) {
	var profile *gmailapi.Profile
	if err := c.withRetry(ctx, "get gmail profile", func() error {
		var err error
		profile, err = c.service.Users.GetProfile("me").Context(ctx).Do()
		return err
	}); err != nil {
		return Profile{}, err
	}
	return Profile{
		EmailAddress: profile.EmailAddress,
		HistoryID:    fmt.Sprintf("%d", profile.HistoryId),
	}, nil
}

func (c *Client) Labels(ctx context.Context) ([]Label, error) {
	var response *gmailapi.ListLabelsResponse
	if err := c.withRetry(ctx, "list gmail labels", func() error {
		var err error
		response, err = c.service.Users.Labels.List("me").Context(ctx).Do()
		return err
	}); err != nil {
		return nil, err
	}

	labels := make([]Label, 0, len(response.Labels))
	for _, label := range response.Labels {
		labels = append(labels, convertLabel(label))
	}
	return labels, nil
}

func convertLabel(label *gmailapi.Label) Label {
	if label == nil {
		return Label{}
	}
	out := Label{
		ID:     label.Id,
		Name:   label.Name,
		Type:   label.Type,
		Unread: int(label.MessagesUnread),
		Total:  int(label.MessagesTotal),
	}
	if label.Color != nil {
		out.ColorBG = label.Color.BackgroundColor
		out.ColorFG = label.Color.TextColor
	}
	return out
}
