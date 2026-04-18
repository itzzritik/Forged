package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type FullPageLoaderScreen struct {
	Title       string
	Description string
}

func RenderFullPageLoader(screen FullPageLoaderScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))

	title := strings.TrimSpace(screen.Title)
	if title == "" {
		title = "Loading"
	}

	lines := []string{
		"",
		"",
		theme.BodyStrong.Width(contentWidth).Align(lipgloss.Center).Render(theme.Spinner.Render(spinner) + " " + title),
	}

	if description := strings.TrimSpace(screen.Description); description != "" {
		lines = append(lines,
			"",
			theme.Body.Width(contentWidth).Align(lipgloss.Center).Render(description),
		)
	}

	lines = append(lines, "", "")
	return strings.Join(lines, "\n")
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
