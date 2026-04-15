package commandui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))
	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	AccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("110"))
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
	WarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("110")).
				Bold(true)
)

func ClampBodyWidth(width int) int {
	switch {
	case width <= 0:
		return 72
	case width < 40:
		return 40
	case width > 72:
		return 72
	default:
		return width
	}
}

func RenderContainer(width int, content string) string {
	bodyWidth := ClampBodyWidth(width - 6)

	style := lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingRight(2).
		Width(bodyWidth)

	return "\n" + style.Render(content) + "\n"
}
