package shell

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type FooterAction struct {
	Key   string
	Label string
}

func RenderFooter(width int, actions ...FooterAction) string {
	prefix := strings.Repeat(" ", ContentLeftInset)
	if len(actions) == 0 {
		return prefix
	}

	full := make([]string, 0, len(actions))
	compact := make([]string, 0, len(actions))
	for _, action := range actions {
		key := theme.FooterKey.Render("[" + action.Key + "]")
		full = append(full, key+" "+theme.FooterLabel.Render(action.Label))
		compact = append(compact, key)
	}

	verbose := prefix + strings.Join(full, "  "+theme.Glyphs.Separator+"  ")
	if width <= 0 || lipgloss.Width(verbose) <= width {
		return verbose
	}

	short := prefix + strings.Join(compact, " ")
	if lipgloss.Width(short) <= width {
		return short
	}

	available := max(0, width-lipgloss.Width(prefix))
	last := compact[len(compact)-1]
	if lipgloss.Width(last) >= available {
		return prefix + last
	}

	parts := make([]string, 0, len(compact))
	used := lipgloss.Width(last)
	omitted := false
	for _, part := range compact[:len(compact)-1] {
		partWidth := lipgloss.Width(part) + 1
		if used+partWidth+2 > available {
			omitted = true
			break
		}
		parts = append(parts, part)
		used += partWidth
	}
	if omitted {
		parts = append(parts, theme.BodyMuted.Render(theme.Glyphs.Ellipsis))
	}
	parts = append(parts, last)
	return prefix + strings.Join(parts, " ")
}
