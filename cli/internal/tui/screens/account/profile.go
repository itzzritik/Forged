package account

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type ProfileScreen struct {
	Name  string
	Email string
}

func RenderProfile(screen ProfileScreen, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	rows := []profileRow{
		{Label: "Name", Value: screen.Name},
		{Label: "Email", Value: screen.Email},
	}

	return renderProfileTable(rows, contentWidth)
}

type profileRow struct {
	Label string
	Value string
}

func renderProfileTable(rows []profileRow, width int) string {
	labelWidth := 8
	valueWidth := max(16, width-labelWidth-2)
	lines := make([]string, 0, len(rows)*2)

	for _, row := range rows {
		value := strings.TrimSpace(row.Value)
		if value == "" {
			value = "—"
		}

		label := padProfileRight(theme.RowLabel.Render(strings.ToUpper(row.Label)), labelWidth+2)
		wrapped := wrapProfileText(value, valueWidth)
		if len(wrapped) == 0 {
			wrapped = []string{"—"}
		}

		lines = append(lines, label+theme.BodyStrong.Render(wrapped[0]))
		for _, line := range wrapped[1:] {
			lines = append(lines, strings.Repeat(" ", labelWidth+2)+theme.BodyStrong.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func padProfileRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	visible := lipgloss.Width(value)
	if visible >= width {
		return value
	}
	return value + strings.Repeat(" ", width-visible)
}

func wrapProfileText(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	if width <= 0 {
		return []string{value}
	}

	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{value}
	}

	lines := make([]string, 0, 2)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
