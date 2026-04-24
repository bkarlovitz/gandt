package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Open     key.Binding
	NextPane key.Binding
	Quit     key.Binding
	Help     key.Binding
	Search   key.Binding
	Compose  key.Binding
	Command  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/k", "nav"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		NextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "pane"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
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
		helpText(k.Down),
		helpText(k.Open),
		helpText(k.NextPane),
		helpText(k.Help),
		helpText(k.Quit),
	}, "   ")
}

func (k KeyMap) HelpText() string {
	return strings.Join([]string{
		k.Down.Help().Key + "  " + k.Down.Help().Desc,
		k.Up.Help().Key + "  " + k.Up.Help().Desc,
		k.Top.Help().Key + "  " + k.Top.Help().Desc,
		k.Bottom.Help().Key + "  " + k.Bottom.Help().Desc,
		k.Open.Help().Key + "  " + k.Open.Help().Desc,
		k.NextPane.Help().Key + "  " + k.NextPane.Help().Desc,
		k.Search.Help().Key + "  " + k.Search.Help().Desc,
		k.Compose.Help().Key + "  " + k.Compose.Help().Desc,
		k.Command.Help().Key + "  " + k.Command.Help().Desc,
		k.Quit.Help().Key + "  " + k.Quit.Help().Desc,
		"Esc  close help",
	}, "\n")
}

func helpText(binding key.Binding) string {
	help := binding.Help()
	separator := ": "
	if strings.HasSuffix(help.Key, ":") {
		separator = " "
	}
	return help.Key + separator + help.Desc
}
