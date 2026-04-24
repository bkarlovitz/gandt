package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	bindings map[string]key.Binding
	actions  map[string]string
}

func DefaultKeyMap() KeyMap {
	return NewKeyMap(nil)
}

func NewKeyMap(overrides map[string]string) KeyMap {
	defs := map[string]struct {
		keys string
		help string
		desc string
	}{
		"up":                      {"k,up", "k/up", "up"},
		"down":                    {"j,down", "j/k", "nav"},
		"top":                     {"g", "g", "top"},
		"bottom":                  {"G", "G", "bottom"},
		"open":                    {"enter", "enter", "open"},
		"next_pane":               {"tab", "tab", "pane"},
		"quit":                    {"q,esc,ctrl+c", "q", "quit"},
		"help":                    {"?", "?", "help"},
		"search":                  {"/", "/", "search"},
		"compose":                 {"c", "c", "compose"},
		"command":                 {":", ":", "command"},
		"thread_next_message":     {"J", "J", "next message"},
		"thread_previous_message": {"K", "K", "prev message"},
		"next_thread":             {"N", "N", "next thread"},
		"previous_thread":         {"P", "P", "prev thread"},
		"render_mode":             {"V", "V", "render"},
		"browser":                 {"B", "B", "browser"},
		"quotes":                  {"z", "z", "quotes"},
		"refresh":                 {"ctrl+r", "ctrl+r", "refresh"},
		"reply":                   {"r", "r", "reply"},
		"reply_all":               {"R", "R", "reply all"},
		"forward":                 {"f", "f", "forward"},
		"archive":                 {"e", "e", "archive"},
		"trash":                   {"#", "#", "trash"},
		"spam":                    {"!", "!", "spam"},
		"star":                    {"s", "s", "star"},
		"unread":                  {"u", "u", "unread"},
		"undo":                    {"U", "U", "undo"},
		"mute":                    {"m", "m", "mute"},
		"label_add":               {"+", "+", "label"},
		"label_remove":            {"-", "-", "unlabel"},
		"account_switcher":        {"ctrl+a", "ctrl+a", "accounts"},
	}
	out := KeyMap{bindings: map[string]key.Binding{}, actions: map[string]string{}}
	for action, def := range defs {
		keys := configSplitKeyList(def.keys)
		if override := strings.TrimSpace(overrides[action]); override != "" {
			keys = configSplitKeyList(override)
		}
		helpKey := def.help
		if len(keys) > 0 && overrideHelpKey(def.help, keys) {
			helpKey = strings.Join(keys, "/")
		}
		binding := key.NewBinding(key.WithKeys(keys...), key.WithHelp(helpKey, def.desc))
		out.bindings[action] = binding
		for _, keyName := range keys {
			out.actions[keyName] = action
		}
	}
	return out
}

func configSplitKeyList(value string) []string {
	return configSplit(value)
}

func configSplit(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func overrideHelpKey(defaultHelp string, keys []string) bool {
	return len(keys) > 0 && !strings.Contains(defaultHelp, keys[0])
}

func (k KeyMap) ActionFor(keyName string) string {
	return k.actions[keyName]
}

func (k KeyMap) Binding(action string) key.Binding {
	return k.bindings[action]
}

func (k KeyMap) Footer() string {
	return strings.Join([]string{
		helpText(k.Binding("down")),
		helpText(k.Binding("open")),
		helpText(k.Binding("next_pane")),
		helpText(k.Binding("help")),
		helpText(k.Binding("quit")),
	}, "   ")
}

func (k KeyMap) HelpText() string {
	actions := []string{
		"down", "up", "top", "bottom", "open", "next_pane",
		"thread_next_message", "thread_previous_message", "next_thread", "previous_thread",
		"search", "compose", "reply", "reply_all", "forward",
		"archive", "trash", "spam", "star", "unread", "undo", "mute", "label_add", "label_remove",
		"render_mode", "browser", "quotes", "refresh", "account_switcher", "command", "quit",
	}
	lines := make([]string, 0, len(actions)+1)
	for _, action := range actions {
		help := k.Binding(action).Help()
		lines = append(lines, help.Key+"  "+help.Desc)
	}
	lines = append(lines, "Esc  close help")
	return strings.Join(lines, "\n")
}

func helpText(binding key.Binding) string {
	help := binding.Help()
	separator := ": "
	if strings.HasSuffix(help.Key, ":") {
		separator = " "
	}
	return help.Key + separator + help.Desc
}
