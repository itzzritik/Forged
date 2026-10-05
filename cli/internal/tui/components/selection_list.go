package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SelectionListItem struct {
	Label    string
	Selected bool
}

func RenderSelectionListPage(items []SelectionListItem, width int, minHeight int, maxHeight int) string {
	if len(items) == 0 || maxHeight <= 0 {
		return ""
	}

	lines := renderSelectionListItems(items, width)
	selected := 0
	for index, item := range items {
		if item.Selected {
			selected = index
			break
		}
	}

	start := 0
	for start < len(lines) {
		end := selectionListPageEnd(lines, start, maxHeight)
		if selected < end {
			return renderSelectionListPage(lines[start:end], minHeight, maxHeight)
		}
		start = end
	}

	return renderSelectionListPage(lines[len(lines)-1:], minHeight, maxHeight)
}

func renderSelectionListItems(items []SelectionListItem, width int) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, renderSelectionListItem(item, width))
	}
	return lines
}

func selectionListPageEnd(items []string, start int, maxHeight int) int {
	used := 0
	end := start
	for end < len(items) {
		itemHeight := max(1, lipgloss.Height(items[end]))
		if end > start && used+itemHeight > maxHeight {
			break
		}
		used += itemHeight
		end++
		if used >= maxHeight {
			break
		}
	}
	return max(start+1, end)
}

func renderSelectionListPage(items []string, minHeight int, maxHeight int) string {
	lines := make([]string, 0, maxHeight)
	for _, item := range items {
		for _, line := range strings.Split(item, "\n") {
			if len(lines) == maxHeight {
				break
			}
			lines = append(lines, line)
		}
		if len(lines) == maxHeight {
			break
		}
	}

	for len(lines) < min(minHeight, maxHeight) {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderSelectionListItem(item SelectionListItem, width int) string {
	prefix := theme.BodyMuted.Render("  ")
	labelStyle := theme.BodyStrong
	if item.Selected {
		prefix = theme.Bullet.Render(theme.Glyphs.Selection + " ")
		labelStyle = theme.Kicker
	}
	return lipgloss.NewStyle().Width(width).Render(prefix + labelStyle.Render(item.Label))
}
