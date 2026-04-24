package ui

import (
	"fmt"
	"strings"

	"github.com/bkarlovitz/gandt/internal/render"
)

type Mailbox struct {
	Account         string
	Labels          []Label
	Messages        []Message
	MessagesByLabel map[string][]Message
	Real            bool
	NoAccounts      bool
	Bootstrapping   bool
	AuthError       string
}

type Label struct {
	ID         string
	Name       string
	Unread     int
	System     bool
	CacheDepth string
}

type Message struct {
	ID              string
	ThreadID        string
	From            string
	Address         string
	Subject         string
	Date            string
	Snippet         string
	Body            []string
	Unread          bool
	Starred         bool
	Muted           bool
	LabelIDs        []string
	ThreadCount     int
	CacheState      string
	AttachmentCount int
	Attachments     []Attachment
	ThreadMessages  []ThreadMessage
}

type ThreadMessage struct {
	ID          string
	From        string
	Address     string
	Date        string
	Body        []string
	Attachments []Attachment
}

type Attachment struct {
	Name string
	Size string
}

func fakeMailbox() Mailbox {
	return Mailbox{
		Account: "work: me@work.com",
		Labels: []Label{
			{Name: "Inbox", Unread: 42, System: true},
			{Name: "Starred", Unread: 3, System: true},
			{Name: "Sent", System: true},
			{Name: "Drafts", Unread: 1, System: true},
			{Name: "Important", Unread: 7, System: true},
			{Name: "Spam", System: true},
			{Name: "Trash", System: true},
			{Name: "receipts", Unread: 4},
			{Name: "travel", System: false},
			{Name: "work", Unread: 12},
		},
		Messages: []Message{
			{
				From:        "Alice",
				Address:     "alice@example.com",
				Subject:     "Re: Q4 plan",
				Date:        "9:21",
				Snippet:     "On Q4, I think we should focus on migration prep and hiring.",
				ThreadCount: 2,
				Unread:      true,
				Body: []string{
					"Hey - on Q4, I think we should focus on the following:",
					"",
					"1. Migration prep",
					"2. Hiring pipeline",
					"3. Customer readiness notes",
				},
				Attachments: []Attachment{
					{Name: "plan.pdf", Size: "142 KB"},
					{Name: "roadmap.png", Size: "88 KB"},
				},
			},
			{
				From:    "Bob",
				Address: "bob@example.com",
				Subject: "Invoice #4132",
				Date:    "9:10",
				Snippet: "Attached the updated invoice for the April services window.",
				Body: []string{
					"Attached the updated invoice for the April services window.",
					"Let me know if you need the purchase order number changed.",
				},
				Attachments: []Attachment{{Name: "invoice-4132.pdf", Size: "64 KB"}},
			},
			{
				From:    "Carol",
				Address: "carol@example.com",
				Subject: "Lunch next week?",
				Date:    "8:55",
				Snippet: "I can do Tuesday or Thursday near the office.",
				Body: []string{
					"I can do Tuesday or Thursday near the office.",
					"Does noon still work for you?",
				},
			},
			{
				From:        "Delta Alerts",
				Address:     "alerts@example.com",
				Subject:     "Build pipeline recovered",
				Date:        "8:31",
				Snippet:     "The nightly Linux build is green again after retry.",
				ThreadCount: 4,
				Body: []string{
					"The nightly Linux build is green again after retry.",
					"No action is required.",
				},
			},
		},
	}
}

func NoAccountMailbox() Mailbox {
	return Mailbox{
		Account:    "no accounts",
		NoAccounts: true,
	}
}

func BootstrappingMailbox() Mailbox {
	mailbox := fakeMailbox()
	mailbox.Bootstrapping = true
	return mailbox
}

func AuthFailureMailbox(message string) Mailbox {
	mailbox := fakeMailbox()
	mailbox.AuthError = message
	return mailbox
}

func RealAccountMailbox(account string, labels []Label, messagesByLabel ...map[string][]Message) Mailbox {
	mailbox := Mailbox{
		Account:  account,
		Labels:   labels,
		Messages: nil,
		Real:     true,
	}
	if len(messagesByLabel) > 0 {
		mailbox.MessagesByLabel = messagesByLabel[0]
		if len(labels) > 0 {
			mailbox.Messages = mailbox.MessagesByLabel[labelKey(labels[0])]
		}
	}
	return mailbox
}

func (m Model) renderMailbox() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	bodyHeight := 12
	if m.height > 5 {
		bodyHeight = m.height - 4
	}

	var body string
	switch {
	case width >= 120:
		labelWidth := 22
		listWidth := 42
		readerWidth := width - labelWidth - listWidth - 6
		body = joinColumns(
			[][]string{
				m.renderLabels(labelWidth, bodyHeight),
				m.renderMessageList(listWidth, bodyHeight),
				m.renderReader(readerWidth, bodyHeight),
			},
			[]int{labelWidth, listWidth, readerWidth},
		)
	case width >= 80:
		listWidth := 38
		readerWidth := width - listWidth - 3
		body = joinColumns(
			[][]string{
				m.renderMessageList(listWidth, bodyHeight),
				m.renderReader(readerWidth, bodyHeight),
			},
			[]int{listWidth, readerWidth},
		)
	default:
		if m.readerOpen || m.focus == PaneReader {
			body = strings.Join(m.renderReader(width, bodyHeight), "\n")
		} else {
			body = strings.Join(m.renderMessageList(width, bodyHeight), "\n")
		}
	}

	header := m.mailboxHeader()
	if m.statusMessage != "" {
		header = fmt.Sprintf("%s | %s", header, m.statusMessage)
	}

	lines := []string{
		fit(header, width),
		strings.Repeat("-", width),
		body,
		strings.Repeat("-", width),
	}
	if m.toastMessage != "" {
		lines = append(lines, fit(m.toastMessage, width), strings.Repeat("-", width))
	}
	lines = append(lines, fit(m.keys.Footer(), width))
	return trimRightLines(strings.Join(lines, "\n"))
}

func (m Model) renderAccountSwitcher() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	lines := []string{"G&T | account switcher", strings.Repeat("-", width)}
	if len(m.accounts) == 0 {
		lines = append(lines, fit("No accounts", width))
	} else {
		for i, account := range m.accounts {
			cursor := " "
			if i == m.activeAccount {
				cursor = ">"
			}
			name := account.DisplayName
			if name == "" {
				name = account.Account
			}
			status := account.SyncStatus
			if status == "" {
				status = "cached"
			}
			color := account.Color
			if color == "" {
				color = "default"
			}
			line := fmt.Sprintf("%s %d  [%s] %s  %s  %d unread", cursor, i+1, color, name, status, account.Unread)
			if account.DisplayName != "" && account.DisplayName != account.Account {
				line += "  " + account.Account
			}
			lines = append(lines, fit(line, width))
		}
	}
	lines = append(lines, strings.Repeat("-", width), fit("Enter switches selected account   1-9 jumps   Esc closes", width))
	return trimRightLines(strings.Join(lines, "\n"))
}

func (m Model) mailboxHeader() string {
	var header string
	switch {
	case m.mailbox.AuthError != "":
		header = fmt.Sprintf("G&T | auth failure: %s | fake inbox | no network", m.mailbox.AuthError)
	case m.mailbox.Bootstrapping:
		header = fmt.Sprintf("G&T | %s | bootstrapping account", m.mailbox.Account)
	case m.mailbox.NoAccounts:
		header = "G&T | no accounts configured"
	case m.mailbox.Real:
		header = fmt.Sprintf("G&T | %s | Gmail cache", m.mailbox.Account)
	default:
		header = fmt.Sprintf("G&T | %s | fake inbox | no network", m.mailbox.Account)
	}
	if m.offline && !strings.Contains(header, "offline") {
		header += " | offline"
	}
	return header
}

func (m Model) renderLabels(width, maxRows int) []string {
	lines := []string{"Labels"}
	if len(m.mailbox.Labels) == 0 {
		lines = append(lines, "", fit("No labels", width))
		return limitLines(lines, maxRows, width)
	}

	userLabelStarted := false
	for i, label := range m.mailbox.Labels {
		if !label.System && !userLabelStarted {
			lines = append(lines, "-- Labels --")
			userLabelStarted = true
		}

		prefix := "  "
		if i == m.selectedLabel {
			prefix = "> "
		}
		count := ""
		if label.Unread > 0 {
			count = fmt.Sprintf("%d", label.Unread)
		}
		if label.CacheDepth == "" {
			lines = append(lines, fit(fmt.Sprintf("%s%-13s %4s", prefix, label.Name, count), width))
			continue
		}
		lines = append(lines, fit(fmt.Sprintf("%s%s %-11s %4s", prefix, depthIndicator(label.CacheDepth), label.Name, count), width))
	}

	return limitLines(lines, maxRows, width)
}

func (m Model) renderMessageList(width, maxRows int) []string {
	currentLabel := "No labels"
	if len(m.mailbox.Labels) > 0 {
		currentLabel = m.mailbox.Labels[m.selectedLabel].Name
	}
	lines := []string{currentLabel}
	if len(m.mailbox.Messages) == 0 {
		lines = append(lines, "", fit("No cached messages", width))
		return limitLines(lines, maxRows, width)
	}
	for i, message := range m.mailbox.Messages {
		prefix := "  "
		if i == m.selectedMessage {
			prefix = "> "
		}
		thread := ""
		if message.ThreadCount > 1 {
			thread = fmt.Sprintf(" [%d]", message.ThreadCount)
		}
		attachment := ""
		if message.AttachmentCount > 0 {
			attachment = " A"
		}
		unread := " "
		if message.Unread {
			unread = "*"
		}
		lines = append(lines,
			m.renderSelectableLine(
				fmt.Sprintf("%s%-12s %5s %s%s %s%s", prefix, message.From, message.Date, message.Subject, thread, unread, attachment),
				width,
				i == m.selectedMessage,
			),
			fit("  "+messageListDetail(message), width),
		)
	}

	return limitLines(lines, maxRows, width)
}

func messageListDetail(message Message) string {
	if message.CacheState == "" {
		return message.Snippet
	}
	return message.CacheState + " | " + message.Snippet
}

func readerDate(value string) string {
	if strings.Contains(value, " ") {
		return value
	}
	if value == "" {
		return ""
	}
	return "Thu Apr 23 " + value
}

func labelKey(label Label) string {
	if label.ID != "" {
		return label.ID
	}
	return label.Name
}

func (m Model) renderReader(width, maxRows int) []string {
	if len(m.mailbox.Messages) == 0 {
		return limitLines([]string{
			fit("Reader", width),
			fit("", width),
			fit("No message selected", width),
		}, maxRows, width)
	}

	message := m.mailbox.Messages[m.selectedMessage]
	readerMessage := m.readerMessage(message)
	lines := []string{
		"Reader",
		"From: " + readerMessage.From + " <" + readerMessage.Address + ">",
		"Subject: " + message.Subject,
		"Date: " + readerDate(readerMessage.Date),
		"",
	}
	switch {
	case m.loadingThreadID != "" && m.loadingThreadID == message.ThreadID:
		lines = append(lines, "Loading thread...")
	case len(readerMessage.Body) > 0:
		lines = append(lines, m.renderBodyLines(readerMessage.Body)...)
	case message.CacheState == "metadata":
		lines = append(lines, "[metadata only; body not cached]")
	default:
		lines = append(lines, "[no body]")
	}
	if len(readerMessage.Attachments) > 0 {
		lines = append(lines, "", fmt.Sprintf("-- %d attachments --", len(readerMessage.Attachments)))
		for _, attachment := range readerMessage.Attachments {
			lines = append(lines, fmt.Sprintf("- %s (%s)", attachment.Name, attachment.Size))
		}
	}

	for i := range lines {
		lines[i] = fit(lines[i], width)
	}
	return limitLines(lines, maxRows, width)
}

func (m Model) readerMessage(message Message) ThreadMessage {
	if len(message.ThreadMessages) > 0 {
		index := clamp(m.selectedThreadMessage, 0, len(message.ThreadMessages)-1)
		return message.ThreadMessages[index]
	}
	return ThreadMessage{
		From:        message.From,
		Address:     message.Address,
		Date:        message.Date,
		Body:        message.Body,
		Attachments: message.Attachments,
	}
}

func (m Model) renderBodyLines(lines []string) []string {
	body := strings.Join(lines, "\n")
	if m.showQuotes {
		return strings.Split(strings.TrimSpace(body), "\n")
	}
	formatted := render.FormatQuotes(body, render.QuoteOptions{CollapseThreshold: 4})
	return strings.Split(formatted, "\n")
}

func depthIndicator(depth string) string {
	switch depth {
	case "full":
		return "F"
	case "body":
		return "B"
	case "metadata":
		return "M"
	case "none":
		return "-"
	default:
		return "?"
	}
}

func (m Model) renderSelectableLine(text string, width int, selected bool) string {
	line := fit(text, width)
	if selected && !m.styles.NoColor {
		return m.styles.Selected.Width(width).Render(line)
	}
	return line
}

func joinColumns(columns [][]string, widths []int) string {
	height := 0
	for _, column := range columns {
		if len(column) > height {
			height = len(column)
		}
	}

	lines := make([]string, 0, height)
	for row := 0; row < height; row++ {
		parts := make([]string, len(columns))
		for col := range columns {
			cell := ""
			if row < len(columns[col]) {
				cell = columns[col][row]
			}
			parts[col] = fit(cell, widths[col])
		}
		lines = append(lines, strings.Join(parts, " | "))
	}

	return strings.Join(lines, "\n")
}

func limitLines(lines []string, maxRows, width int) []string {
	if maxRows <= 0 || len(lines) <= maxRows {
		return lines
	}
	if maxRows == 1 {
		return []string{fit("...", width)}
	}

	limited := append([]string{}, lines[:maxRows-1]...)
	limited = append(limited, fit("...", width))
	return limited
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > width {
		if width <= 3 {
			return string(runes[:width])
		}
		return string(runes[:width-3]) + "..."
	}
	return value + strings.Repeat(" ", width-len(runes))
}

func trimRightLines(value string) string {
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}
