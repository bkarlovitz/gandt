package gmail

import (
	"context"

	gmailapi "google.golang.org/api/gmail/v1"
)

type SendAsIdentity struct {
	Email              string
	DisplayName        string
	ReplyToAddress     string
	IsDefault          bool
	IsPrimary          bool
	VerificationStatus string
}

func (c *Client) ListSendAs(ctx context.Context) ([]SendAsIdentity, error) {
	var response *gmailapi.ListSendAsResponse
	if err := c.withRetry(ctx, "list gmail send-as identities", func() error {
		var err error
		response, err = c.service.Users.Settings.SendAs.List("me").Context(ctx).Do()
		return err
	}); err != nil {
		return nil, err
	}

	identities := make([]SendAsIdentity, 0, len(response.SendAs))
	for _, identity := range response.SendAs {
		converted := convertSendAsIdentity(identity)
		if converted.Email == "" {
			continue
		}
		identities = append(identities, converted)
	}
	return identities, nil
}

func convertSendAsIdentity(identity *gmailapi.SendAs) SendAsIdentity {
	if identity == nil {
		return SendAsIdentity{}
	}
	return SendAsIdentity{
		Email:              identity.SendAsEmail,
		DisplayName:        identity.DisplayName,
		ReplyToAddress:     identity.ReplyToAddress,
		IsDefault:          identity.IsDefault,
		IsPrimary:          identity.IsPrimary,
		VerificationStatus: identity.VerificationStatus,
	}
}
