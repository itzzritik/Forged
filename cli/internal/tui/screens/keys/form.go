package keys

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type RenameScreen struct {
	Context string
	FieldView string
	Focused bool
	Status  string
	Error   string
	Loading bool
}

type DeleteScreen struct {
	Context string
	Key     actions.KeySummary
	Status  string
	Error   string
	Loading bool
}

func RenderRename(screen RenameScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	sections := make([]string, 0, 4)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Loading {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" Loading key"))
		return strings.Join(sections, "\n")
	}

	sections = append(sections, "", renderTextField(screen.FieldView, screen.Focused))
	if status := renderStatus(screen.Status, screen.Error, spinner); status != "" {
		sections = append(sections, status)
	} else {
		sections = append(sections, "")
	}
	return strings.Join(sections, "\n")
}

func RenderDelete(screen DeleteScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	sections := make([]string, 0, 4)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Loading {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" Loading key"))
		return strings.Join(sections, "\n")
	}

	lines := []string{
		renderDetailRow("Name", screen.Key.Name),
		renderDetailRow("Type", strings.ToUpper(screen.Key.Type)),
		renderDetailRow("Fingerprint", screen.Key.Fingerprint),
	}
	sections = append(sections, "", strings.Join(lines, "\n"))

	if status := renderStatus(screen.Status, screen.Error, spinner); status != "" {
		sections = append(sections, "", status)
	}

	return strings.Join(sections, "\n")
}

func renderTextField(view string, focused bool) string {
	lineStyle := theme.FieldLineIdle
	if focused {
		lineStyle = theme.FieldLineActive
	}
	renderedValue := view
	width := max(18, lipgloss.Width(strings.TrimSpace(view)))
	width = min(max(width+2, 24), 44)
	return strings.Join([]string{
		"",
		renderedValue,
		lineStyle.Render(strings.Repeat("─", width)),
	}, "\n")
}

func renderStatus(info string, err string, spinner string) string {
	if strings.TrimSpace(err) != "" {
		return theme.Danger.Render("✕ " + err)
	}
	if strings.TrimSpace(info) != "" {
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " " + info)
	}
	return ""
}
