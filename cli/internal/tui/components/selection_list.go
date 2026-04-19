package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SelectionListItem struct {
	Label    string
	Selected bool
}

func RenderSelectionList(items []SelectionListItem, width int, minHeight int) string {
	if len(items) == 0 {
		return ""
	}

	lines := make([]string, 0, max(minHeight, len(items)))
	for _, item := range items {
		lines = append(lines, renderSelectionListItem(item, width))
	}
	for len(lines) < minHeight {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderSelectionListItem(item SelectionListItem, width int) string {
	prefix := theme.BodyMuted.Render("  ")
	labelStyle := theme.BodyStrong
	if item.Selected {
		prefix = theme.Bullet.Render("▸ ")
		labelStyle = theme.Kicker
	}
	return lipgloss.NewStyle().Width(width).Render(prefix + labelStyle.Render(item.Label))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
