package account

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type ProfileScreen struct {
	Name  string
	Email string
}

func RenderProfile(screen ProfileScreen, width int) string {
	contentWidth := max(16, min(width, theme.HeroMaxWidth))
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
	if width < 28 {
		return renderStackedProfileRows(rows, width)
	}

	labelWidth := 8
	valueWidth := width - labelWidth - 2
	lines := make([]string, 0, len(rows)*2)

	for _, row := range rows {
		value := profileValue(row.Value)

		label := padProfileRight(theme.RowLabel.Render(strings.ToUpper(row.Label)), labelWidth+2)
		wrapped := wrapProfileText(value, valueWidth)
		if len(wrapped) == 0 {
			wrapped = []string{theme.Glyphs.Empty}
		}

		lines = append(lines, label+theme.BodyStrong.Render(wrapped[0]))
		for _, line := range wrapped[1:] {
			lines = append(lines, strings.Repeat(" ", labelWidth+2)+theme.BodyStrong.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func renderStackedProfileRows(rows []profileRow, width int) string {
	lines := make([]string, 0, len(rows)*3)
	for index, row := range rows {
		lines = append(lines, theme.RowLabel.Render(strings.ToUpper(row.Label)))
		for _, line := range wrapProfileText(profileValue(row.Value), max(1, width)) {
			lines = append(lines, theme.BodyStrong.Render(line))
		}
		if index < len(rows)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func profileValue(value string) string {
	value = strings.TrimSpace(theme.SanitizeText(value))
	if value == "" {
		return theme.Glyphs.Empty
	}
	return value
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

	return strings.Split(ansi.Hardwrap(value, width, false), "\n")
}
