package ui

import "github.com/bkarlovitz/gandt/internal/compose"

type DraftComposeState struct {
	Account string
	DraftID compose.DraftID
	Headers compose.Headers
	Body    string
}

func ReopenDraftMessage(accountID string, accountEmail string, message Message) DraftComposeState {
	return DraftComposeState{
		Account: accountEmail,
		DraftID: compose.DraftID{
			GmailDraftID:   message.ID,
			GmailMessageID: message.ID,
			ThreadID:       message.ThreadID,
		},
		Headers: compose.Headers{
			ActiveAccountID: accountID,
			AccountEmail:    accountEmail,
			SendAs:          compose.NewAddress(accountEmail),
			Subject:         message.Subject,
		},
		Body: joinMessageBody(message.Body),
	}
}

func joinMessageBody(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}
