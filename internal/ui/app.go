package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/bkarlovitz/gandt/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeSearch
	ModeCompose
	ModeCommand
	ModeHelp
)

type Pane int

const (
	PaneLabels Pane = iota
	PaneList
	PaneReader
)

type Model struct {
	config          config.Config
	keys            KeyMap
	styles          Styles
	mailbox         Mailbox
	mode            Mode
	width           int
	height          int
	focus           Pane
	selectedLabel   int
	selectedMessage int
	readerOpen      bool
	quitting        bool
	commandInput    string
	statusMessage   string
	addingAccount   bool
	addAccount      AccountAdder
}

type Option func(*Model)

func WithAccountAdder(adder AccountAdder) Option {
	return func(m *Model) {
		if adder != nil {
			m.addAccount = adder
		}
	}
}

func WithMailbox(mailbox Mailbox) Option {
	return func(m *Model) {
		m.mailbox = mailbox
	}
}

func New(cfg config.Config, opts ...Option) Model {
	model := Model{
		config:  cfg,
		keys:    DefaultKeyMap(),
		styles:  NewStyles(os.Getenv("NO_COLOR") != ""),
		mailbox: fakeMailbox(),
		mode:    ModeNormal,
		focus:   PaneList,
	}
	for _, opt := range opts {
		opt(&model)
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case addAccountDoneMsg:
		m.addingAccount = false
		if msg.Err != nil {
			m.statusMessage = "add account failed: " + msg.Err.Error()
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("added account %s", msg.Result.Account)
		if len(msg.Result.Labels) > 0 {
			m.mailbox = RealAccountMailbox(msg.Result.Account, msg.Result.Labels)
			m.selectedLabel = clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)
		}
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.mode == ModeNormal {
		return m.renderMailbox()
	}

	var b strings.Builder
	b.WriteString(m.styles.Header.Render("G&T"))
	b.WriteString("\n\n")

	switch m.mode {
	case ModeHelp:
		b.WriteString("Help\n\n")
		b.WriteString(m.keys.HelpText())
	case ModeSearch:
		b.WriteString("Search mode\n\nPress Esc to return.")
	case ModeCompose:
		b.WriteString("Compose mode\n\nPress Esc to return.")
	case ModeCommand:
		b.WriteString("Command mode\n\n")
		b.WriteString(m.commandInput)
		b.WriteString("\n\nPress Esc to return.")
	}

	return b.String()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.mode {
	case ModeHelp:
		switch key {
		case "esc", "?":
			m.mode = ModeNormal
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case ModeSearch, ModeCompose, ModeCommand:
		if m.mode == ModeCommand {
			return m.handleCommandKey(msg)
		}
		if key == "esc" {
			m.mode = ModeNormal
		}
		return m, nil
	}

	switch key {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = ModeHelp
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "g":
		m.jumpSelection(false)
	case "G":
		m.jumpSelection(true)
	case "enter":
		m.readerOpen = true
		m.focus = PaneReader
	case "tab":
		m.nextPane()
	case "/":
		m.mode = ModeSearch
	case "c", "r", "R", "f":
		m.mode = ModeCompose
	case ":":
		m.mode = ModeCommand
		m.commandInput = ":"
	}

	return m, nil
}

func (m Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ModeNormal
		m.commandInput = ""
		return m, nil
	case tea.KeyEnter:
		return m.submitCommand()
	case tea.KeyBackspace, tea.KeyCtrlH:
		if len(m.commandInput) > 1 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
		return m, nil
	}

	key := msg.String()
	if key == "esc" {
		m.mode = ModeNormal
		m.commandInput = ""
		return m, nil
	}
	if key == "backspace" || key == "ctrl+h" {
		if len(m.commandInput) > 1 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
		return m, nil
	}
	for _, r := range msg.Runes {
		m.commandInput += string(r)
	}
	return m, nil
}

func (m Model) submitCommand() (tea.Model, tea.Cmd) {
	command := strings.TrimSpace(strings.TrimPrefix(m.commandInput, ":"))
	m.mode = ModeNormal
	m.commandInput = ""

	switch command {
	case "add-account":
		if m.addingAccount {
			m.statusMessage = "add account already running"
			return m, nil
		}
		if m.addAccount == nil {
			m.statusMessage = "add account unavailable"
			return m, nil
		}
		m.addingAccount = true
		m.statusMessage = "adding account..."
		return m, m.runAddAccount()
	case "quit", "q":
		m.quitting = true
		return m, tea.Quit
	case "":
		return m, nil
	default:
		m.statusMessage = "unknown command: " + command
		return m, nil
	}
}

func (m Model) runAddAccount() tea.Cmd {
	return func() tea.Msg {
		result, err := m.addAccount.AddAccount()
		return addAccountDoneMsg{Result: result, Err: err}
	}
}

func (m *Model) moveSelection(delta int) {
	switch m.focus {
	case PaneLabels:
		m.selectedLabel = clamp(m.selectedLabel+delta, 0, len(m.mailbox.Labels)-1)
	default:
		m.selectedMessage = clamp(m.selectedMessage+delta, 0, len(m.mailbox.Messages)-1)
	}
}

func (m *Model) jumpSelection(bottom bool) {
	target := 0
	if bottom {
		switch m.focus {
		case PaneLabels:
			target = len(m.mailbox.Labels) - 1
		default:
			target = len(m.mailbox.Messages) - 1
		}
	}

	switch m.focus {
	case PaneLabels:
		m.selectedLabel = clamp(target, 0, len(m.mailbox.Labels)-1)
	default:
		m.selectedMessage = clamp(target, 0, len(m.mailbox.Messages)-1)
	}
}

func (m *Model) nextPane() {
	if m.width > 0 && m.width < 80 {
		if m.focus == PaneReader {
			m.focus = PaneList
			m.readerOpen = false
			return
		}
		m.focus = PaneReader
		m.readerOpen = true
		return
	}

	if m.width >= 120 {
		switch m.focus {
		case PaneLabels:
			m.focus = PaneList
		case PaneList:
			m.focus = PaneReader
			m.readerOpen = true
		default:
			m.focus = PaneLabels
		}
		return
	}

	if m.focus == PaneReader {
		m.focus = PaneList
		return
	}
	m.focus = PaneReader
	m.readerOpen = true
}

func clamp(value, min, max int) int {
	if max < min {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
