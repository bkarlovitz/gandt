package auth

import (
	"context"

	"github.com/charmbracelet/huh"
)

type HuhCredentialPrompt struct{}

func (HuhCredentialPrompt) PromptClientCredentials(ctx context.Context) (ClientCredentials, error) {
	var clientID string
	var clientSecret string

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Google OAuth client ID").
			Value(&clientID),
		huh.NewInput().
			Title("Google OAuth client secret").
			EchoMode(huh.EchoModePassword).
			Value(&clientSecret),
	))
	if err := form.RunWithContext(ctx); err != nil {
		return ClientCredentials{}, err
	}

	return ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, nil
}
