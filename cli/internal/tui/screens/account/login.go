package account

import (
	"net/url"
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
	left := renderPrimaryPane(screen, spinner, width)
	right := renderUtilityRail(screen, width)
	if right == "" {
		return left
	}

	if width >= 68 {
		railWidth := min(34, max(28, width/3))
		leftWidth := max(30, width-railWidth-3)
		leftBlock := lipgloss.NewStyle().Width(leftWidth).Render(left)
		rightBlock := theme.AsideRail.Width(railWidth).Render(right)
		return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, "   ", rightBlock)
	}

	return strings.Join([]string{
		left,
		"",
		theme.AsideRail.Render(right),
	}, "\n")
}

func renderPrimaryPane(screen LoginScreen, spinner string, width int) string {
	lines := make([]string, 0, 4)
	if strings.TrimSpace(screen.Context) != "" {
		lines = append(lines, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}
	if status := renderStatus(screen, spinner); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}
	if screen.Error != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderError(screen.Error, min(52, max(28, width-6))))
	}
	return strings.Join(lines, "\n")
}

func renderUtilityRail(screen LoginScreen, width int) string {
	sections := make([]string, 0, 3)

	if screen.VerificationCode != "" {
		sections = append(sections, renderCode(screen.VerificationCode))
	}
	if screen.URL != "" {
		sections = append(sections, renderLink(screen.URL, max(22, min(30, width/3)), screen.Copied))
	}

	return strings.Join(sections, "\n\n")
}

func renderCode(code string) string {
	return strings.Join([]string{
		theme.SectionTitle.Render("Verification code"),
		theme.CodeFrame.Render(strings.Join([]string{
			theme.CodeLabel.Render("CODE"),
			theme.CodeValue.Render(code),
		}, "\n")),
	}, "\n")
}

func renderLink(raw string, width int, copied bool) string {
	host := raw
	target := raw
	if parsed, err := url.Parse(raw); err == nil {
		if parsed.Scheme != "" && parsed.Host != "" {
			host = parsed.Scheme + "://" + parsed.Host
		}
		if requestURI := parsed.RequestURI(); requestURI != "" {
			target = requestURI
		}
	}

	lines := []string{
		theme.SectionTitle.Render("Open link"),
		theme.LinkHost.Render(host),
		theme.LinkPath.Width(max(18, width)).Render(target),
	}
	if copied {
		lines = append(lines, theme.Success.Render("✓ Link copied"))
	}
	return strings.Join(lines, "\n")
}

func renderStatus(screen LoginScreen, spinner string) string {
	if screen.Waiting {
		label := screen.Status
		if strings.TrimSpace(label) == "" {
			label = "Waiting for approval"
		}
		return theme.BodyStrong.Render(spinner + " " + label)
	}
	if strings.TrimSpace(screen.Status) != "" {
		return theme.BodyStrong.Render(screen.Status)
	}
	return ""
}

func renderError(message string, width int) string {
	title := "Sign-in could not start"
	detail := message

	switch {
	case strings.Contains(message, "could not reach server"):
		title = "Unable to reach the sign-in service"
		detail = "Check connectivity, then open the approval link again."
	case strings.Contains(message, "timed out"):
		title = "Approval timed out"
		detail = "Open the link again and finish browser approval before the session expires."
	}

	lines := []string{
		theme.Danger.Render(title),
		theme.Body.Width(max(24, width)).Render(detail),
	}
	return theme.AlertRail.Width(max(30, width+3)).Render(strings.Join(lines, "\n"))
}
