package ui

import (
	"fmt"
	"strings"

	"github.com/bkarlovitz/gandt/internal/compose"
	tea "github.com/charmbracelet/bubbletea"
)

type ComposeModeState struct {
	Kind           compose.ComposeKind
	Headers        compose.Headers
	Body           string
	Attachments    []string
	SendStatus     compose.SendStatus
	AutosaveStatus string
	Error          string
	DiscardConfirm bool
	AttachPrompt   bool
}

func composeKindNew() compose.ComposeKind      { return compose.ComposeKindNew }
func composeKindReply() compose.ComposeKind    { return compose.ComposeKindReply }
func composeKindReplyAll() compose.ComposeKind { return compose.ComposeKindReplyAll }
func composeKindForward() compose.ComposeKind  { return compose.ComposeKindForward }

func (m *Model) startComposeMode(kind compose.ComposeKind) {
	original := m.selectedComposeOriginal()
	headers := compose.Headers{
		ActiveAccountID: m.mailbox.Account,
		AccountEmail:    m.mailbox.Account,
		SendAs:          compose.NewAddress(m.mailbox.Account),
	}
	body := ""
	switch kind {
	case compose.ComposeKindReply:
		ctx := compose.NewReplyContext(original, compose.NewAddress(m.mailbox.Account), false)
		headers.To = ctx.Recipients()
		headers.Subject = ctx.Subject()
		body = "\n\n" + compose.ReplyQuote(original)
	case compose.ComposeKindReplyAll:
		ctx := compose.NewReplyContext(original, compose.NewAddress(m.mailbox.Account), true)
		headers.To = ctx.Recipients()
		headers.Subject = ctx.Subject()
		body = "\n\n" + compose.ReplyQuote(original)
	case compose.ComposeKindForward:
		ctx := compose.NewForwardContext(original)
		headers.Subject = ctx.Subject()
		body = "\n\n" + compose.ForwardQuote(original)
	default:
		kind = compose.ComposeKindNew
	}
	m.compose = ComposeModeState{
		Kind:           kind,
		Headers:        headers,
		Body:           body,
		SendStatus:     compose.SendStatusEditing,
		AutosaveStatus: "not saved",
	}
	m.mode = ModeCompose
	m.statusMessage = fmt.Sprintf("compose %s", strings.ReplaceAll(string(kind), "_", "-"))
}

func (m Model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.compose.DiscardConfirm {
		switch key {
		case "y", "Y":
			m.compose = ComposeModeState{}
			m.mode = ModeNormal
			m.statusMessage = "compose discarded"
		case "n", "N", "esc":
			m.compose.DiscardConfirm = false
			m.statusMessage = "discard canceled"
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyCtrlD:
		m.compose.AutosaveStatus = "draft saved"
		m.compose.SendStatus = compose.SendStatusDraftSaved
		m.mode = ModeNormal
		m.statusMessage = "draft saved"
	case tea.KeyCtrlS:
		m.compose.SendStatus = compose.SendStatusSent
		m.compose.AutosaveStatus = "sent"
		m.mode = ModeNormal
		m.statusMessage = "send complete"
	case tea.KeyCtrlC:
		m.compose.DiscardConfirm = true
		m.statusMessage = "discard compose? y/n"
	case tea.KeyCtrlT:
		m.compose.AttachPrompt = true
		m.statusMessage = "attach file"
	case tea.KeyEsc:
		m.mode = ModeNormal
		m.statusMessage = "compose closed"
	}
	return m, nil
}

func (m Model) selectedComposeOriginal() compose.OriginalMessage {
	message := m.selectedMailboxMessage()
	return compose.OriginalMessage{
		AccountID: m.mailbox.Account,
		MessageID: message.ID,
		ThreadID:  message.ThreadID,
		From:      compose.NewAddress(addressOrName(message.Address, message.From)),
		Subject:   message.Subject,
		Date:      m.now(),
		BodyPlain: strings.Join(message.Body, "\n"),
	}
}

func (m Model) selectedMailboxMessage() Message {
	messages := m.currentMessages()
	if len(messages) == 0 {
		return Message{}
	}
	return messages[clamp(m.selectedMessage, 0, len(messages)-1)]
}

func addressOrName(address string, name string) string {
	if strings.TrimSpace(address) != "" {
		return address
	}
	return name
}

func (m Model) renderComposeMode() string {
	lines := []string{
		fmt.Sprintf("Compose %s", strings.ReplaceAll(string(m.compose.Kind), "_", "-")),
		"",
		"From: " + m.compose.Headers.SendAs.String(),
		"To: " + formatComposeAddressList(m.compose.Headers.To),
		"Subject: " + m.compose.Headers.Subject,
		"Status: " + string(m.compose.SendStatus),
		"Autosave: " + m.compose.AutosaveStatus,
	}
	if len(m.compose.Attachments) > 0 {
		lines = append(lines, "Attachments: "+strings.Join(m.compose.Attachments, ", "))
	}
	if m.compose.AttachPrompt {
		lines = append(lines, "Attach: path input")
	}
	if m.compose.Error != "" {
		lines = append(lines, "Error: "+m.compose.Error)
	}
	if m.compose.DiscardConfirm {
		lines = append(lines, "Discard? y/n")
	}
	if strings.TrimSpace(m.compose.Body) != "" {
		lines = append(lines, "", m.compose.Body)
	}
	return strings.Join(lines, "\n")
}
