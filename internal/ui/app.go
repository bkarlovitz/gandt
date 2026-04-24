package ui

import (
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

type Model struct {
	config   config.Config
	keys     KeyMap
	styles   Styles
	mode     Mode
	width    int
	height   int
	quitting bool
}

func New(cfg config.Config) Model {
	return Model{
		config: cfg,
		keys:   DefaultKeyMap(),
		styles: NewStyles(),
		mode:   ModeNormal,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
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
		b.WriteString("Command mode\n\nPress Esc to return.")
	default:
		b.WriteString("Fake mailbox coming in Sprint 1.\n\n")
		b.WriteString(m.keys.Footer())
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
	case "/":
		m.mode = ModeSearch
	case "c", "r", "R", "f":
		m.mode = ModeCompose
	case ":":
		m.mode = ModeCommand
	}

	return m, nil
}
