package compose

import (
	"fmt"
	"strings"
)

func ReplyQuote(original OriginalMessage) string {
	return strings.Join([]string{
		fmt.Sprintf("On %s, %s wrote:", formatQuoteDate(original), original.From.String()),
		quoteLines(original.BodyPlain),
	}, "\n")
}

func ForwardQuote(original OriginalMessage) string {
	lines := []string{
		"---------- Forwarded message ---------",
		"From: " + original.From.String(),
		"Date: " + formatQuoteDate(original),
	}
	if len(original.To) > 0 {
		lines = append(lines, "To: "+formatAddressList(original.To))
	}
	if len(original.Cc) > 0 {
		lines = append(lines, "Cc: "+formatAddressList(original.Cc))
	}
	if strings.TrimSpace(original.Subject) != "" {
		lines = append(lines, "Subject: "+strings.TrimSpace(original.Subject))
	}
	lines = append(lines, "", quoteLines(original.BodyPlain))
	return strings.Join(lines, "\n")
}

func quoteLines(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

func formatQuoteDate(original OriginalMessage) string {
	if original.Date.IsZero() {
		return "unknown date"
	}
	return original.Date.Format("Jan 2, 2006 at 3:04 PM")
}

func formatAddressList(addresses []Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address.Email) == "" {
			continue
		}
		formatted = append(formatted, address.String())
	}
	return strings.Join(formatted, ", ")
}
