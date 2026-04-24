package ui

import (
	"fmt"
	"strings"
)

type Mailbox struct {
	Account  string
	Labels   []Label
	Messages []Message
}

type Label struct {
	Name   string
	Unread int
	System bool
}

type Message struct {
	From        string
	Address     string
	Subject     string
	Date        string
	Snippet     string
	Body        []string
	Unread      bool
	ThreadCount int
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
		if m.readerOpen {
			body = strings.Join(m.renderReader(width, bodyHeight), "\n")
		} else {
			body = strings.Join(m.renderMessageList(width, bodyHeight), "\n")
		}
	}

	return trimRightLines(strings.Join([]string{
		fit(fmt.Sprintf("G&T | %s | fake inbox | no network", m.mailbox.Account), width),
		strings.Repeat("-", width),
		body,
		strings.Repeat("-", width),
		fit(m.keys.Footer(), width),
	}, "\n"))
}

func (m Model) renderLabels(width, maxRows int) []string {
	lines := []string{"Labels"}
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
		lines = append(lines, fit(fmt.Sprintf("%s%-13s %4s", prefix, label.Name, count), width))
	}

	return limitLines(lines, maxRows, width)
}

func (m Model) renderMessageList(width, maxRows int) []string {
	currentLabel := m.mailbox.Labels[m.selectedLabel].Name
	lines := []string{currentLabel}
	for i, message := range m.mailbox.Messages {
		prefix := "  "
		if i == m.selectedMessage {
			prefix = "> "
		}
		thread := ""
		if message.ThreadCount > 1 {
			thread = fmt.Sprintf(" [%d]", message.ThreadCount)
		}
		unread := " "
		if message.Unread {
			unread = "*"
		}
		lines = append(lines,
			m.renderSelectableLine(
				fmt.Sprintf("%s%-12s %5s %s%s %s", prefix, message.From, message.Date, message.Subject, thread, unread),
				width,
				i == m.selectedMessage,
			),
			fit("  "+message.Snippet, width),
		)
	}

	return limitLines(lines, maxRows, width)
}

func (m Model) renderReader(width, maxRows int) []string {
	message := m.mailbox.Messages[m.selectedMessage]
	lines := []string{
		"Reader",
		"From: " + message.From + " <" + message.Address + ">",
		"Subject: " + message.Subject,
		"Date: Thu Apr 23 " + message.Date,
		"",
	}
	lines = append(lines, message.Body...)
	if len(message.Attachments) > 0 {
		lines = append(lines, "", fmt.Sprintf("-- %d attachments --", len(message.Attachments)))
		for _, attachment := range message.Attachments {
			lines = append(lines, fmt.Sprintf("- %s (%s)", attachment.Name, attachment.Size))
		}
	}

	for i := range lines {
		lines[i] = fit(lines[i], width)
	}
	return limitLines(lines, maxRows, width)
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
