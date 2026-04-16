package account

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	const leftInset = 2

	lines := make([]string, 0, 8)
	contentWidth := max(28, min(width, theme.HeroMaxWidth))

	if screen.Error != "" {
		return indentLines(renderError(screen.Error, contentWidth), leftInset)
	}

	if strings.TrimSpace(screen.Context) != "" {
		lines = append(lines, theme.Body.Width(contentWidth).Render(screen.Context))
	}

	if code := renderCode(screen.VerificationCode); code != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, code)
	}

	if status := renderStatus(screen, spinner); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}

	if link := renderLink(screen.URL, screen.Copied); link != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, link)
	}

	return indentLines(strings.Join(lines, "\n"), leftInset)
}

func renderCode(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}

	innerWidth := max(13, lipgloss.Width(code)+4)
	inner := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center).
		Render(theme.CodeValue.Render(code))
	return theme.CodeFrame.Render(inner)
}

func renderLink(raw string, copied bool) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	label := theme.BodyStrong.Render("Login URL")
	if copied {
		label += "  " + theme.Success.Render("✓") + " " + theme.BodyMuted.Render("Copied")
	}
	return strings.Join([]string{
		label,
		theme.Link.Render(raw),
	}, "\n")
}

func renderStatus(screen LoginScreen, spinner string) string {
	if screen.Waiting {
		label := screen.Status
		if strings.TrimSpace(label) == "" {
			label = "Waiting for browser approval"
		}
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " " + label)
	}
	if strings.TrimSpace(screen.Status) != "" {
		return theme.BodyStrong.Render(screen.Status)
	}
	return ""
}

func renderError(message string, width int) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}

	title := "Sign-in could not start"
	detail := message

	switch {
	case strings.Contains(message, "could not reach server"):
		title = "Unable to reach the sign-in service"
		detail = "Check connectivity, then open the link again."
	case strings.Contains(message, "timed out"):
		title = "Approval timed out"
		detail = "Open the link again to continue."
	}

	lines := []string{
		theme.Danger.Render("✕ " + title),
	}
	if strings.TrimSpace(detail) != "" {
		lines = append(lines, theme.Body.Width(max(24, width)).Render(detail))
	}
	return strings.Join(lines, "\n")
}

func indentLines(block string, spaces int) string {
	if strings.TrimSpace(block) == "" || spaces <= 0 {
		return block
	}

	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(block, "\n")
	for index, line := range lines {
		if line == "" {
			continue
		}
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}
