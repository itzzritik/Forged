package keys

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
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
	Warning string
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
	Failed      bool
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
	Failure     string
	Error       string
	Status      string
	Busy        bool
}

type ExportScreen struct {
	Context     string
	Warning     string
	PathView    string
	Focused     bool
	PathVisible bool
	Status      string
	Error       string
	Busy        bool
}

func RenderRename(screen RenameScreen, spinner string, width int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	fieldWidth := inputFieldWidth(contentWidth)
	top := make([]string, 0, 3)
	if context := strings.TrimSpace(screen.Context); context != "" {
		top = append(top, renderFormText(theme.Body, context, contentWidth))
	}

	if screen.Loading {
		return shell.DockBottom(strings.Join(top, "\n"), renderFormText(theme.BodyStrong, theme.Spinner.Render(spinner)+" Loading key", contentWidth))
	}

	top = append(top, "", renderTextField(screen.FieldView, screen.Focused, fieldWidth))
	return shell.DockBottom(strings.Join(top, "\n"), renderStatus(screen.Status, screen.Error, spinner, contentWidth))
}

func RenderDelete(screen DeleteScreen, spinner string, width int, bodyHeight int, scrollOffset int) ViewportRender {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	sections := make([]string, 0, 4)
	if context := strings.TrimSpace(screen.Context); context != "" {
		sections = append(sections, theme.Body.Width(contentWidth).Render(context))
	}

	if screen.Loading {
		sections = append(sections, "", theme.BodyStrong.Width(contentWidth).Render(theme.Spinner.Render(spinner)+" Loading key"))
		return renderViewport(strings.Join(sections, "\n"), "", bodyHeight, scrollOffset)
	}

	rows := []detailTableRow{
		{Label: "Name", Value: screen.Key.Name, Style: theme.BodyStrong},
		{Label: "Type", Value: strings.ToUpper(screen.Key.Type), Style: theme.BodyStrong},
		{Label: "Fingerprint", Value: screen.Key.Fingerprint, Style: theme.BodyStrong},
	}
	sections = append(sections, "", renderDetailTable(rows, contentWidth))

	feedback := ""
	switch {
	case strings.TrimSpace(screen.Error) != "":
		feedback = renderStatus("", screen.Error, spinner, contentWidth)
	case strings.TrimSpace(screen.Status) != "":
		feedback = renderStatus(screen.Status, "", spinner, contentWidth)
	case strings.TrimSpace(screen.Warning) != "":
		feedback = renderFormText(theme.Warning, "! "+displayMessage(screen.Warning), contentWidth)
	}
	return renderViewport(strings.Join(sections, "\n"), feedback, bodyHeight, scrollOffset)
}

func RenderGenerate(screen GenerateScreen, spinner string, width int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	fieldWidth := inputFieldWidth(contentWidth)
	top := make([]string, 0, 3)
	if context := strings.TrimSpace(screen.Context); context != "" {
		top = append(top, renderFormText(theme.Body, context, contentWidth))
	}

	if screen.Generating {
		return shell.DockBottom(strings.Join(top, "\n"), renderFormText(theme.BodyStrong, theme.Spinner.Render(spinner)+" "+screen.Status, contentWidth))
	}

	top = append(top,
		"",
		renderTextField(screen.NameView, screen.Focused, fieldWidth),
	)
	return shell.DockBottom(strings.Join(top, "\n"), renderResultStatus(screen.Status, "", screen.Error, false, spinner, contentWidth))
}

func RenderImport(screen ImportScreen, spinner string, width int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	fieldWidth := max(1, min(contentWidth, 54))
	top := make([]string, 0, 5)
	if context := strings.TrimSpace(screen.Context); context != "" {
		top = append(top, renderFormText(theme.Body, context, contentWidth))
	}

	if screen.Busy {
		return shell.DockBottom(strings.Join(top, "\n"), renderFormText(theme.BodyStrong, theme.Spinner.Render(spinner)+" "+screen.Status, contentWidth))
	}

	if len(screen.Sources) > 0 {
		lines := make([]string, 0, len(screen.Sources))
		for _, source := range screen.Sources {
			prefix := theme.BodyMuted.Render(theme.Glyphs.Bullet)
			labelStyle := theme.BodyMuted
			if source.Selected {
				prefix = theme.Kicker.Render(theme.Glyphs.Selection)
				labelStyle = theme.BodyStrong
			}
			labelWidth := contentWidth - lipgloss.Width(prefix) - 1
			if labelWidth <= 0 {
				lines = append(lines, ansi.Truncate(prefix, contentWidth, theme.Glyphs.Ellipsis))
				continue
			}
			lines = append(lines, prefix+" "+labelStyle.Render(ansi.Truncate(source.Label, labelWidth, theme.Glyphs.Ellipsis)))
		}
		top = append(top, "", strings.Join(lines, "\n"))
	}

	if screen.PathVisible {
		top = append(top, "", renderTextField(screen.PathView, screen.PathFocused, fieldWidth))
	}

	return shell.DockBottom(strings.Join(top, "\n"), renderResultStatus(screen.Status, screen.Warning, screen.Error, false, spinner, contentWidth))
}

func RenderImportReview(screen ImportReviewScreen, spinner string, width int, height int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth+10))
	items := append([]ImportReviewItem(nil), screen.Items...)
	active := 0
	for index, item := range items {
		if item.Active {
			active = index
			break
		}
	}
	hasAbove := screen.HasAbove
	hasBelow := screen.HasBelow
	showContext := true
	showSource := true
	showMarkers := true
	compactRows := false
	showSummaryHeading := true
	summary := append([]string(nil), screen.Summary...)
	showGuidance := true
	compactFeedback := false
	bottom := renderImportReviewBottom(screen, spinner, contentWidth, showSummaryHeading, summary, showGuidance, compactFeedback)
	top := renderImportReviewTop(screen, items, hasAbove, hasBelow, contentWidth, showContext, showSource, showMarkers, compactRows)
	trimFarthestItem := func() {
		left := active
		right := len(items) - active - 1
		if right >= left {
			items = items[:len(items)-1]
			hasBelow = true
		} else {
			items = items[1:]
			active--
			hasAbove = true
		}
		top = renderImportReviewTop(screen, items, hasAbove, hasBelow, contentWidth, showContext, showSource, showMarkers, compactRows)
	}
	for height > 0 && len(items) > 3 && importReviewBlockHeight(top)+importReviewBlockHeight(bottom) > height {
		trimFarthestItem()
	}
	for _, compact := range []func(){
		func() { showContext = false },
		func() { showSource = false },
		func() { showGuidance = false },
		func() { showMarkers = false },
		func() { compactFeedback = true },
		func() { showSummaryHeading = false },
	} {
		if height <= 0 || importReviewBlockHeight(top)+importReviewBlockHeight(bottom) <= height {
			break
		}
		compact()
		top = renderImportReviewTop(screen, items, hasAbove, hasBelow, contentWidth, showContext, showSource, showMarkers, compactRows)
		bottom = renderImportReviewBottom(screen, spinner, contentWidth, showSummaryHeading, summary, showGuidance, compactFeedback)
	}
	for height > 0 && len(items) > 1 && importReviewBlockHeight(top)+importReviewBlockHeight(bottom) > height {
		trimFarthestItem()
	}
	for height > 0 && len(summary) > 0 && importReviewBlockHeight(top)+importReviewBlockHeight(bottom) > height {
		summary = summary[1:]
		bottom = renderImportReviewBottom(screen, spinner, contentWidth, showSummaryHeading, summary, showGuidance, compactFeedback)
	}
	if height > 0 && importReviewBlockHeight(top)+importReviewBlockHeight(bottom) > height {
		compactRows = true
		top = renderImportReviewTop(screen, items, hasAbove, hasBelow, contentWidth, showContext, showSource, showMarkers, compactRows)
	}
	if height > 0 && !showSummaryHeading && len(summary) > 0 {
		withHeading := renderImportReviewBottom(screen, spinner, contentWidth, true, summary, showGuidance, compactFeedback)
		if importReviewBlockHeight(top)+importReviewBlockHeight(withHeading) <= height {
			bottom = withHeading
		}
	}
	return shell.DockBottom(top, bottom)
}

func renderImportReviewTop(screen ImportReviewScreen, items []ImportReviewItem, hasAbove bool, hasBelow bool, width int, showContext bool, showSource bool, showMarkers bool, compactRows bool) string {
	sections := make([]string, 0, 3)
	if context := strings.TrimSpace(screen.Context); showContext && context != "" {
		sections = append(sections, theme.Body.Width(width).Render(context))
	}

	lines := make([]string, 0, len(items)+3)
	if showSource {
		source := fmt.Sprintf("%s %s %d keys", screen.SourceLabel, theme.Glyphs.Separator, screen.Count)
		lines = append(lines, theme.BodyMuted.Render(ansi.Truncate(source, width, theme.Glyphs.Ellipsis)))
	}
	if showMarkers && hasAbove {
		lines = append(lines, theme.BodyMuted.Render(theme.Glyphs.Up+" more"))
	}
	for _, item := range items {
		if compactRows {
			lines = append(lines, renderImportReviewCompactRow(item, width))
		} else {
			lines = append(lines, renderImportReviewRow(item, width))
		}
	}
	if showMarkers && hasBelow {
		lines = append(lines, theme.BodyMuted.Render(theme.Glyphs.Down+" more"))
	}
	if len(sections) > 0 {
		sections = append(sections, "")
	}
	sections = append(sections, strings.Join(lines, "\n"))
	return strings.Join(sections, "\n")
}

func renderImportReviewBottom(screen ImportReviewScreen, spinner string, width int, showSummaryHeading bool, summary []string, showGuidance bool, compactFeedback bool) string {
	lines := make([]string, 0, 7)
	if showSummaryHeading && len(summary) > 0 {
		lines = append(lines, theme.SectionTitle.Render("Summary"))
	}
	for _, line := range summary {
		lines = append(lines, theme.BodyMuted.Render(ansi.Truncate(line, width, theme.Glyphs.Ellipsis)))
	}
	if guidance := strings.TrimSpace(screen.Guidance); showGuidance && guidance != "" {
		lines = append(lines, theme.BodyMuted.Width(width).Render(guidance))
	}
	switch {
	case strings.TrimSpace(screen.Error) != "":
		err := strings.TrimSpace(screen.Error)
		lines = append(lines, renderImportReviewFeedback(theme.Danger, theme.Glyphs.Cross+" "+displayMessage(err), width, compactFeedback))
	case screen.Busy && strings.TrimSpace(screen.Status) != "":
		lines = append(lines, renderImportReviewFeedback(theme.BodyStrong, spinner+" "+displayMessage(screen.Status), width, compactFeedback))
	case strings.TrimSpace(screen.Warning) != "":
		warning := strings.TrimSpace(screen.Warning)
		lines = append(lines, renderImportReviewFeedback(theme.Warning, "! "+displayMessage(warning), width, compactFeedback))
	case strings.TrimSpace(screen.Status) != "":
		lines = append(lines, renderImportReviewFeedback(theme.BodyStrong, displayMessage(screen.Status), width, compactFeedback))
	}
	if failure := strings.TrimSpace(screen.Failure); failure != "" {
		lines = append(lines, renderImportReviewFeedback(theme.Danger, theme.Glyphs.Cross+" "+displayMessage(failure), width, compactFeedback))
	}
	return strings.Join(lines, "\n")
}

func renderImportReviewFeedback(style lipgloss.Style, message string, width int, compact bool) string {
	if compact {
		return style.Render(truncateImportReviewLine(strings.Join(strings.Fields(message), " "), width))
	}
	return style.Width(width).Render(message)
}

func truncateImportReviewLine(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)+theme.Glyphs.Ellipsis) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + theme.Glyphs.Ellipsis
}

func importReviewBlockHeight(block string) int {
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}

func RenderExport(screen ExportScreen, spinner string, width int) string {
	contentWidth := max(1, min(width, theme.HeroMaxWidth))
	fieldWidth := max(1, min(contentWidth, 54))
	top := make([]string, 0, 3)
	if context := strings.TrimSpace(screen.Context); context != "" {
		top = append(top, renderFormText(theme.Body, context, contentWidth))
	}

	if screen.Busy {
		return shell.DockBottom(strings.Join(top, "\n"), renderExportFeedback(screen, spinner, contentWidth))
	}

	if screen.PathVisible {
		top = append(top, "", renderTextField(screen.PathView, screen.Focused, fieldWidth))
	}
	return shell.DockBottom(strings.Join(top, "\n"), renderExportFeedback(screen, spinner, contentWidth))
}

func renderExportFeedback(screen ExportScreen, spinner string, width int) string {
	sections := make([]string, 0, 2)
	if warning := strings.TrimSpace(screen.Warning); warning != "" {
		sections = append(sections, renderFormText(theme.Warning, "! "+warning, width))
	}
	if status := renderResultStatus(screen.Status, "", screen.Error, screen.Busy, spinner, width); status != "" {
		sections = append(sections, status)
	}
	return strings.Join(sections, "\n")
}

func renderTextField(view string, focused bool, width int) string {
	lineStyle := theme.FieldLineIdle
	if focused {
		lineStyle = theme.FieldLineActive
	}
	fieldWidth := max(1, width)
	renderedValue := lipgloss.NewStyle().Width(fieldWidth).Render(view)
	return strings.Join([]string{
		renderedValue,
		lineStyle.Render(strings.Repeat(theme.Glyphs.Horizontal, fieldWidth)),
	}, "\n")
}

func renderFormText(style lipgloss.Style, value string, width int) string {
	width = max(1, width)
	if width < 28 {
		return style.Render(ansi.Truncate(value, width, theme.Glyphs.Ellipsis))
	}
	return style.Width(width).Render(value)
}

func renderImportReviewRow(item ImportReviewItem, width int) string {
	return strings.Join([]string{
		renderImportReviewCompactRow(item, width),
		ansi.Truncate("    "+renderImportMetadataLine(item), width, theme.Glyphs.Ellipsis),
	}, "\n")
}

func renderImportReviewCompactRow(item ImportReviewItem, width int) string {
	prefix := " "
	if item.Active {
		prefix = theme.Kicker.Render(theme.Glyphs.Selection)
	}
	prefix = fmt.Sprintf("%s %s ", prefix, renderImportCheckbox(item))
	return prefix + ansi.Truncate(theme.SanitizeText(item.Name), max(0, width-lipgloss.Width(prefix)), theme.Glyphs.Ellipsis)
}

func renderImportCheckbox(item ImportReviewItem) string {
	if item.Failed {
		return theme.Danger.Render(theme.Glyphs.Cross)
	}
	if item.Checked {
		return theme.Kicker.Render(theme.Glyphs.Checked)
	}
	return theme.BodyMuted.Render(theme.Glyphs.Unchecked)
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
	if item.Failed {
		badges = append(badges, theme.Danger.Render("Failed"))
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
	return max(1, min(contentWidth, 44))
}

func renderStatus(info string, err string, spinner string, width int) string {
	width = max(1, width)
	if strings.TrimSpace(err) != "" {
		return renderFormText(theme.Danger, theme.Glyphs.Cross+" "+displayMessage(err), width)
	}
	if strings.TrimSpace(info) != "" {
		return renderFormText(theme.BodyStrong, theme.Spinner.Render(spinner)+" "+displayMessage(info), width)
	}
	return ""
}

func renderResultStatus(info string, warning string, err string, busy bool, spinner string, width int) string {
	width = max(1, width)
	if strings.TrimSpace(err) != "" {
		return renderFormText(theme.Danger, theme.Glyphs.Cross+" "+displayMessage(err), width)
	}
	if strings.TrimSpace(warning) != "" {
		return renderFormText(theme.Warning, "! "+displayMessage(warning), width)
	}
	if strings.TrimSpace(info) == "" {
		return ""
	}
	if busy {
		return renderFormText(theme.BodyStrong, theme.Spinner.Render(spinner)+" "+displayMessage(info), width)
	}
	return renderFormText(theme.Success, theme.Glyphs.Check+" "+displayMessage(info), width)
}

func displayMessage(value string) string {
	trimmed := strings.TrimSpace(theme.SanitizeText(value))
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
