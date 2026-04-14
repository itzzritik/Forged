package launcher

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("110"))
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("110")).
				Bold(true)
)
