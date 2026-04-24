package ui

import (
	"strings"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	NoColor  bool
	Theme    config.Theme
	Header   lipgloss.Style
	Selected lipgloss.Style
	Accent   lipgloss.Style
	Unread   lipgloss.Style
	Read     lipgloss.Style
	Muted    lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
}

type palette struct {
	primary    string
	accent     string
	selectedBG string
	selectedFG string
	muted      string
	error      string
	warning    string
	success    string
	read       string
	unread     string
}

func NewStyles(theme config.Theme, noColor bool) Styles {
	p := paletteFor(theme)
	styles := Styles{
		NoColor: noColor,
		Theme:   theme,
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.primary)),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(p.selectedBG)).
			Foreground(lipgloss.Color(p.selectedFG)),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.accent)),
		Unread: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.unread)),
		Read: lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color(p.read)),
		Muted: lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color(p.muted)),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.warning)),
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(p.error)),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(p.success)),
	}
	if noColor {
		styles.Header = lipgloss.NewStyle()
		styles.Selected = lipgloss.NewStyle()
		styles.Accent = lipgloss.NewStyle()
		styles.Unread = lipgloss.NewStyle()
		styles.Read = lipgloss.NewStyle()
		styles.Muted = lipgloss.NewStyle()
		styles.Warning = lipgloss.NewStyle()
		styles.Error = lipgloss.NewStyle()
		styles.Success = lipgloss.NewStyle()
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

func (s Styles) Status(value string) string {
	if s.NoColor || value == "" {
		return value
	}
	style := s.statusStyle(value)
	return style.Render(value)
}

func (s Styles) statusStyle(value string) lipgloss.Style {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "failed"), strings.Contains(lower, "error"), strings.Contains(lower, "fatal"), strings.Contains(lower, "revoked"):
		return s.Error
	case strings.Contains(lower, "warn"), strings.Contains(lower, "offline"), strings.Contains(lower, "quota"), strings.Contains(lower, "rate"), strings.Contains(lower, "confirm"):
		return s.Warning
	case strings.Contains(lower, "complete"), strings.Contains(lower, "saved"), strings.Contains(lower, "loaded"), strings.Contains(lower, "added"), strings.Contains(lower, "removed"), strings.Contains(lower, "sent"), strings.Contains(lower, "synced"), strings.Contains(lower, "opened"):
		return s.Success
	default:
		return s.Muted
	}
}

func paletteFor(theme config.Theme) palette {
	switch theme {
	case config.ThemeLight:
		return palette{
			primary:    "#1f2937",
			accent:     "#087f8c",
			selectedBG: "#d9f2ef",
			selectedFG: "#102a2a",
			muted:      "#667085",
			error:      "#b42318",
			warning:    "#b54708",
			success:    "#067647",
			read:       "#667085",
			unread:     "#111827",
		}
	case config.ThemeAuto:
		fallthrough
	default:
		return palette{
			primary:    "#d7f8f3",
			accent:     "#6bcfbd",
			selectedBG: "#245c5a",
			selectedFG: "#f7fffd",
			muted:      "#8ea3a1",
			error:      "#ff6b6b",
			warning:    "#f4b942",
			success:    "#58d68d",
			read:       "#8ea3a1",
			unread:     "#ffffff",
		}
	}
}
