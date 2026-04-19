package common

import (
	"strings"

	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SuccessScreen struct {
	Context string
	Title   string
	Message string
	Detail  string
}

func RenderSuccess(screen SuccessScreen, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	sections := make([]string, 0, 8)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	confetti := strings.Join([]string{
		theme.Kicker.Render("✦"),
		theme.Success.Render("✓"),
		theme.Kicker.Render("✦"),
	}, "   ")
	sections = append(sections,
		"",
		confetti,
		"",
		theme.HeroTitle.Width(contentWidth).Render(screen.Title),
	)
	if message := strings.TrimSpace(screen.Message); message != "" {
		sections = append(sections, theme.Success.Width(contentWidth).Render(screen.Message))
	}
	if detail := strings.TrimSpace(screen.Detail); detail != "" {
		sections = append(sections, "", theme.BodyMuted.Width(contentWidth).Render(detail))
	}

	return strings.Join(sections, "\n")
}
