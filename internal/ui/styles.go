package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	NoColor  bool
	Header   lipgloss.Style
	Selected lipgloss.Style
	Accent   lipgloss.Style
}

func NewStyles(noColor bool) Styles {
	styles := Styles{
		NoColor: noColor,
		Header:  lipgloss.NewStyle().Bold(true),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f6feb")).
			Foreground(lipgloss.Color("#ffffff")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1f6feb")),
	}
	if noColor {
		styles.Header = lipgloss.NewStyle()
		styles.Selected = lipgloss.NewStyle()
		styles.Accent = lipgloss.NewStyle()
	}
	return styles
}

func (s Styles) WithAccent(color string) Styles {
	if s.NoColor || color == "" {
		return s
	}
	s.Header = s.Header.Foreground(lipgloss.Color(color))
	s.Selected = s.Selected.Background(lipgloss.Color(color))
	s.Accent = s.Accent.Foreground(lipgloss.Color(color))
	return s
}
