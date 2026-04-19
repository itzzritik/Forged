package keys

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type RenameScreen struct {
	Context   string
	FieldView string
	Focused   bool
	Status    string
	Error     string
	Loading   bool
}

type DeleteScreen struct {
	Context string
	Key     actions.KeySummary
	Status  string
	Error   string
	Loading bool
}

type GenerateScreen struct {
	Context    string
	NameView   string
	Focused    bool
	Status     string
	Error      string
	Generating bool
}

type ImportSourceOption struct {
	Label    string
	Selected bool
}

type ImportScreen struct {
	Context     string
	Sources     []ImportSourceOption
	SourceFocus bool
	PathView    string
	PathFocused bool
	PathVisible bool
	Status      string
	Warning     string
	Error       string
	Busy        bool
}

type ImportReviewItem struct {
	Name        string
	Fingerprint string
	Checked     bool
	Active      bool
	Converted   bool
}

type ImportReviewScreen struct {
	Context     string
	SourceLabel string
	Count       int
	Items       []ImportReviewItem
	HasAbove    bool
	HasBelow    bool
	Summary     []string
	Guidance    string
	Warning     string
	Error       string
}

type ExportScreen struct {
	Context     string
	PathView    string
	Focused     bool
	PathVisible bool
	Status      string
	Error       string
	Busy        bool
}

type TransferSuccessScreen struct {
	Context string
	Title   string
	Message string
	Detail  string
}

func RenderRename(screen RenameScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	fieldWidth := inputFieldWidth(contentWidth)
	sections := make([]string, 0, 4)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Loading {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" Loading key"))
		return strings.Join(sections, "\n")
	}

	sections = append(sections, "", renderTextField(screen.FieldView, screen.Focused, fieldWidth))
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

func RenderGenerate(screen GenerateScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	fieldWidth := inputFieldWidth(contentWidth)
	sections := make([]string, 0, 6)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Generating {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" "+screen.Status))
		return strings.Join(sections, "\n")
	}

	sections = append(sections,
		"",
		renderTextField(screen.NameView, screen.Focused, fieldWidth),
	)
	if status := renderResultStatus(screen.Status, "", screen.Error, false, spinner); status != "" {
		sections = append(sections, "")
		sections = append(sections, status)
	} else {
		sections = append(sections, "")
	}
	return strings.Join(sections, "\n")
}

func RenderImport(screen ImportScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	fieldWidth := max(28, min(contentWidth, 54))
	sections := make([]string, 0, 6)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Busy {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" "+screen.Status))
		return strings.Join(sections, "\n")
	}

	if len(screen.Sources) > 0 {
		lines := make([]string, 0, len(screen.Sources))
		for _, source := range screen.Sources {
			prefix := theme.BodyMuted.Render("·")
			labelStyle := theme.BodyMuted
			if source.Selected {
				prefix = theme.Kicker.Render("▸")
				labelStyle = theme.BodyStrong
			}
			lines = append(lines, prefix+" "+labelStyle.Render(source.Label))
		}
		sections = append(sections, "", strings.Join(lines, "\n"))
	}

	if screen.PathVisible {
		sections = append(sections, "", renderTextField(screen.PathView, screen.PathFocused, fieldWidth))
	}

	if status := renderResultStatus(screen.Status, screen.Warning, screen.Error, false, spinner); status != "" {
		sections = append(sections, "", status)
	} else {
		sections = append(sections, "")
	}

	return strings.Join(sections, "\n")
}

func RenderImportReview(screen ImportReviewScreen, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))
	sections := make([]string, 0, 8)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	lines := []string{
		theme.BodyMuted.Render(screen.SourceLabel),
		theme.BodyMuted.Render(fmt.Sprintf("%d keys found", screen.Count)),
		"",
	}
	if screen.HasAbove {
		lines = append(lines, theme.BodyMuted.Render("..."), "")
	}
	for _, item := range screen.Items {
		lines = append(lines, renderImportReviewRow(item), "")
	}
	if screen.HasBelow {
		lines = append(lines, theme.BodyMuted.Render("..."), "")
	}

	if len(screen.Summary) > 0 {
		lines = append(lines, theme.SectionTitle.Render("Summary"))
		for _, line := range screen.Summary {
			lines = append(lines, theme.BodyMuted.Render(line))
		}
	}
	if guidance := strings.TrimSpace(screen.Guidance); guidance != "" {
		lines = append(lines, "", theme.BodyMuted.Width(contentWidth).Render(guidance))
	}
	if warning := strings.TrimSpace(screen.Warning); warning != "" {
		lines = append(lines, "", theme.Warning.Width(contentWidth).Render("! "+displayMessage(warning)))
	}
	if err := strings.TrimSpace(screen.Error); err != "" {
		lines = append(lines, "", theme.Danger.Width(contentWidth).Render("✕ "+displayMessage(err)))
	}

	sections = append(sections, "", strings.Join(lines, "\n"))
	return strings.Join(sections, "\n")
}

func RenderExport(screen ExportScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth))
	fieldWidth := max(28, min(contentWidth, 54))
	sections := make([]string, 0, 5)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Busy {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" "+screen.Status))
		return strings.Join(sections, "\n")
	}

	if screen.PathVisible {
		sections = append(sections, "", renderTextField(screen.PathView, screen.Focused, fieldWidth))
	}
	if status := renderResultStatus(screen.Status, "", screen.Error, false, spinner); status != "" {
		sections = append(sections, "")
		sections = append(sections, status)
	} else {
		sections = append(sections, "")
	}
	return strings.Join(sections, "\n")
}

func RenderTransferSuccess(screen TransferSuccessScreen, width int) string {
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

func renderTextField(view string, focused bool, width int) string {
	lineStyle := theme.FieldLineIdle
	if focused {
		lineStyle = theme.FieldLineActive
	}
	fieldWidth := max(24, width)
	renderedValue := lipgloss.NewStyle().Width(fieldWidth).Render(view)
	return strings.Join([]string{
		"",
		renderedValue,
		lineStyle.Render(strings.Repeat("─", fieldWidth)),
	}, "\n")
}

func renderImportReviewRow(item ImportReviewItem) string {
	prefix := " "
	if item.Active {
		prefix = theme.Kicker.Render("▸")
	}

	firstLine := fmt.Sprintf("%s %s %s", prefix, renderImportCheckbox(item.Checked), item.Name)
	lines := []string{
		firstLine,
		"    " + renderImportMetadataLine(item),
	}
	return strings.Join(lines, "\n")
}

func renderImportCheckbox(checked bool) string {
	if checked {
		return theme.Kicker.Render("■")
	}
	return theme.BodyMuted.Render("□")
}

func renderImportMetadataLine(item ImportReviewItem) string {
	parts := []string{theme.BodyMuted.Render(truncateImportFingerprint(item.Fingerprint))}
	if badges := renderImportBadges(item); badges != "" {
		parts = append(parts, badges)
	}
	return strings.Join(parts, theme.BodyMuted.Render(" | "))
}

func renderImportBadges(item ImportReviewItem) string {
	var badges []string
	if item.Converted {
		badges = append(badges, theme.Kicker.Render("Upgrade"))
	}
	return strings.Join(badges, theme.BodyMuted.Render(" | "))
}

func truncateImportFingerprint(value string) string {
	if len(value) <= 20 {
		return value
	}
	return value[:13] + "..." + value[len(value)-4:]
}

func inputFieldWidth(contentWidth int) int {
	return max(28, min(contentWidth, 44))
}

func renderStatus(info string, err string, spinner string) string {
	if strings.TrimSpace(err) != "" {
		return theme.Danger.Render("✕ " + displayMessage(err))
	}
	if strings.TrimSpace(info) != "" {
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " " + displayMessage(info))
	}
	return ""
}

func renderResultStatus(info string, warning string, err string, busy bool, spinner string) string {
	if strings.TrimSpace(err) != "" {
		return theme.Danger.Render("✕ " + displayMessage(err))
	}
	if strings.TrimSpace(warning) != "" {
		return theme.Warning.Render("! " + displayMessage(warning))
	}
	if strings.TrimSpace(info) == "" {
		return ""
	}
	if busy {
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " " + displayMessage(info))
	}
	return theme.Success.Render("✓ " + displayMessage(info))
}

func displayMessage(value string) string {
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
