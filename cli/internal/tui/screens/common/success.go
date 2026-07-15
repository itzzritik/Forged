package common

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SuccessScreen struct {
	Context string
	Title   string
	Message string
	Detail  string
	Warning string
}

func RenderSuccess(screen SuccessScreen, width int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	top := make([]string, 0, 5)
	if context := strings.TrimSpace(screen.Context); context != "" {
		if contentWidth < 28 {
			top = append(top, theme.Body.Render(ansi.Truncate(context, contentWidth, theme.Glyphs.Ellipsis)))
		} else {
			top = append(top, theme.Body.Width(contentWidth).Render(context))
		}
	}

	confetti := strings.Join([]string{
		theme.Kicker.Render(theme.Glyphs.Sparkle),
		theme.Success.Render(theme.Glyphs.Check),
		theme.Kicker.Render(theme.Glyphs.Sparkle),
	}, "   ")
	title := theme.HeroTitle.Width(contentWidth).Render(screen.Title)
	if contentWidth < 28 {
		title = theme.HeroTitle.Render(ansi.Truncate(screen.Title, contentWidth, theme.Glyphs.Ellipsis))
	}
	top = append(top,
		"",
		confetti,
		"",
		title,
	)

	if contentWidth >= 28 {
		sections := append([]string(nil), top...)
		if message := strings.TrimSpace(screen.Message); message != "" {
			sections = append(sections, theme.Success.Width(contentWidth).Render(message))
		}
		if detail := strings.TrimSpace(screen.Detail); detail != "" {
			sections = append(sections, "", theme.BodyMuted.Width(contentWidth).Render(detail))
		}
		if warning := strings.TrimSpace(screen.Warning); warning != "" {
			sections = append(sections, "", theme.Warning.Width(contentWidth).Render("! "+warning))
		}
		return strings.Join(sections, "\n")
	}

	bottom := make([]string, 0, 3)
	if message := strings.TrimSpace(screen.Message); message != "" {
		bottom = append(bottom, theme.Success.Render(ansi.Truncate(message, contentWidth, theme.Glyphs.Ellipsis)))
	}
	if detail := strings.TrimSpace(screen.Detail); detail != "" {
		bottom = append(bottom, theme.BodyMuted.Render(ansi.Truncate(detail, contentWidth, theme.Glyphs.Ellipsis)))
	}
	if warning := strings.TrimSpace(screen.Warning); warning != "" {
		bottom = append(bottom, theme.Warning.Render(ansi.Truncate("! "+warning, contentWidth, theme.Glyphs.Ellipsis)))
	}

	return shell.DockBottom(strings.Join(top, "\n"), strings.Join(bottom, "\n"))
}
