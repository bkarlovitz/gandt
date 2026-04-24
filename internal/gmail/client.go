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
}

func NewClient(ctx context.Context, opts ...option.ClientOption) (*Client, error) {
	service, err := gmailapi.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Client{service: service}, nil
}

func (c *Client) Profile(ctx context.Context) (Profile, error) {
	profile, err := c.service.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return Profile{}, fmt.Errorf("get gmail profile: %w", err)
	}
	return Profile{
		EmailAddress: profile.EmailAddress,
		HistoryID:    fmt.Sprintf("%d", profile.HistoryId),
	}, nil
}

func (c *Client) Labels(ctx context.Context) ([]Label, error) {
	response, err := c.service.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list gmail labels: %w", err)
	}

	labels := make([]Label, 0, len(response.Labels))
	for _, label := range response.Labels {
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
		labels = append(labels, out)
	}
	return labels, nil
}
