package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	Header lipgloss.Style
}

func NewStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().Bold(true),
	}
}
