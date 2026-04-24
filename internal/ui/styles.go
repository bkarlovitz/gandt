package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	NoColor  bool
	Header   lipgloss.Style
	Selected lipgloss.Style
}

func NewStyles(noColor bool) Styles {
	styles := Styles{
		NoColor: noColor,
		Header:  lipgloss.NewStyle().Bold(true),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f6feb")).
			Foreground(lipgloss.Color("#ffffff")),
	}
	if noColor {
		styles.Header = lipgloss.NewStyle()
		styles.Selected = lipgloss.NewStyle()
	}
	return styles
}
