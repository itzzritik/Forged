package shell

import (
	"strings"

	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type FooterAction struct {
	Key   string
	Label string
}

func RenderFooter(actions ...FooterAction) string {
	if len(actions) == 0 {
		return ""
	}

	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		parts = append(parts, theme.FooterKey.Render("["+action.Key+"]")+" "+theme.FooterLabel.Render(action.Label))
	}
	return strings.Repeat(" ", ContentLeftInset) + strings.Join(parts, "  ·  ")
}
