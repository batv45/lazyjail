package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds every style the UI uses. Colors are ANSI 16 so they work in any
// terminal; the plain theme (NO_COLOR / --no-color) uses only bold and reverse.
type Theme struct {
	Title, Dim, Banned, Active, Warn, Err lipgloss.Style
	Cursor, Checked                       lipgloss.Style
	Border, BorderFocus                   lipgloss.Style
}

// NewTheme returns the default theme, or a colorless one when plain is true.
func NewTheme(plain bool) Theme {
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if plain {
		bold := lipgloss.NewStyle().Bold(true)
		return Theme{
			Title: bold, Cursor: lipgloss.NewStyle().Reverse(true), Checked: bold,
			Banned: bold, Active: lipgloss.NewStyle(), Warn: bold, Err: bold, Dim: lipgloss.NewStyle(),
			Border: box, BorderFocus: box.Border(lipgloss.ThickBorder()),
		}
	}
	return Theme{
		Title:       lipgloss.NewStyle().Bold(true),
		Dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Banned:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		Active:      lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		Warn:        lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		Err:         lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Cursor:      lipgloss.NewStyle().Reverse(true),
		Checked:     lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true),
		Border:      box.BorderForeground(lipgloss.Color("8")),
		BorderFocus: box.BorderForeground(lipgloss.Color("12")),
	}
}
