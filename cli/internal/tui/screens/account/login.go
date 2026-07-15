package account

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type LoginScreen struct {
	Title            string
	Context          string
	Status           string
	VerificationCode string
	URL              string
	Waiting          bool
	Copied           bool
	Error            string
}

func Render(screen LoginScreen, spinner string, width int) string {
	lines := make([]string, 0, 8)
	contentWidth := max(1, min(width, theme.HeroMaxWidth))

	if screen.Error != "" {
		return renderError(screen.Error, contentWidth)
	}

	if strings.TrimSpace(screen.Context) != "" {
		lines = append(lines, theme.Body.Width(contentWidth).Render(screen.Context))
	}

	if code := renderCode(screen.VerificationCode, contentWidth); code != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, code)
	}

	if status := renderStatus(screen, spinner, contentWidth); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}

	if link := renderLink(screen.URL, screen.Copied, contentWidth); link != "" {
		return shell.DockBottom(strings.Join(lines, "\n"), link)
	}

	return strings.Join(lines, "\n")
}

func renderCode(code string, width int) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}

	innerWidth := min(max(1, width-theme.CodeFrame.GetHorizontalFrameSize()), max(13, lipgloss.Width(code)+4))
	inner := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center).
		Render(theme.CodeValue.Render(code))
	return theme.CodeFrame.Render(inner)
}

func renderLink(raw string, copied bool, width int) string {
	url := strings.TrimSpace(theme.SanitizeText(raw))
	if url == "" {
		return ""
	}

	width = max(1, width)
	label := theme.BodyStrong.Render("Log In Link")
	if copied {
		label = shell.JoinRow(width, label, theme.Success.Render(theme.Glyphs.Check)+" "+theme.BodyMuted.Render("Copied"))
	}
	return strings.Join([]string{
		label,
		theme.Link.Render(ansi.Hardwrap(url, width, false)),
	}, "\n")
}

func renderStatus(screen LoginScreen, spinner string, width int) string {
	if screen.Waiting {
		label := screen.Status
		if strings.TrimSpace(label) == "" {
			label = "Waiting for browser approval"
		}
		return theme.BodyStrong.Width(width).Render(theme.Spinner.Render(spinner) + " " + label)
	}
	if strings.TrimSpace(screen.Status) != "" {
		return theme.BodyStrong.Width(width).Render(screen.Status)
	}
	return ""
}

func renderError(message string, width int) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}

	title := "Unable to start log-in flow"
	detail := sentenceCase(message)
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "could not reach server"):
		title = "Unable to reach the log-in service"
		detail = "Check connectivity, then open the link again."
	case strings.Contains(lower, "timed out"):
		title = "Approval timed out"
		detail = "Open the link again to continue."
	}

	lines := []string{theme.Danger.Width(width).Render(theme.Glyphs.Cross + " " + title)}
	if strings.TrimSpace(detail) != "" {
		lines = append(lines, theme.Body.Width(width).Render(detail))
	}
	return strings.Join(lines, "\n")
}

func sentenceCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) == 0 || !unicode.IsLower(runes[0]) {
		return trimmed
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
