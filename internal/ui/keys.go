package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Search  key.Binding
	Compose key.Binding
	Command key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q/esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Compose: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "compose"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
		),
	}
}

func (k KeyMap) Footer() string {
	return strings.Join([]string{
		k.Help.Help().Key + ": " + k.Help.Help().Desc,
		k.Search.Help().Key + ": " + k.Search.Help().Desc,
		k.Compose.Help().Key + ": " + k.Compose.Help().Desc,
		k.Command.Help().Key + ": " + k.Command.Help().Desc,
		k.Quit.Help().Key + ": " + k.Quit.Help().Desc,
	}, "   ")
}

func (k KeyMap) HelpText() string {
	return strings.Join([]string{
		k.Search.Help().Key + "  " + k.Search.Help().Desc,
		k.Compose.Help().Key + "  " + k.Compose.Help().Desc,
		k.Command.Help().Key + "  " + k.Command.Help().Desc,
		k.Quit.Help().Key + "  " + k.Quit.Help().Desc,
		"Esc  close help",
	}, "\n")
}
