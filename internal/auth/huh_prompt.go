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
		huh.NewNote().
			Title("Google OAuth setup").
			Description("Use one Google Desktop OAuth client for all Gmail accounts. Enable Gmail API in Google Cloud, then paste the client ID and secret here."),
		huh.NewInput().
			Title("Google OAuth client ID").
			Description("From Google Auth platform > Clients > Desktop app. Stored in the OS keychain.").
			Value(&clientID),
		huh.NewInput().
			Title("Google OAuth client secret").
			Description("Stored in the OS keychain and never written to config.toml.").
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

func (HuhCredentialPrompt) ConfirmClientCredentialReplacement(ctx context.Context) (bool, error) {
	var confirmed bool

	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Replace Google OAuth client credentials?").
			Description("Future Gmail authorization will use the new client credentials.").
			Affirmative("Replace").
			Negative("Cancel").
			Value(&confirmed),
	))
	if err := form.RunWithContext(ctx); err != nil {
		return false, err
	}

	return confirmed, nil
}
