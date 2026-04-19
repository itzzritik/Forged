package keys

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type DetailScreen struct {
	Loading     bool
	Error       string
	Key         actions.KeyDetail
	Status      string
	StatusError string
	Busy        bool
}

func RenderDetail(screen DetailScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))
	if screen.Loading {
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Looking up key",
			Description: "Checking the vault for a matching key",
		}, spinner, contentWidth)
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		return theme.Danger.Render("✕ " + displayMessage(msg))
	}

	name := strings.TrimSpace(screen.Key.Name)
	if name == "" {
		name = "Unnamed key"
	}

	sections := []string{
		theme.SectionTitle.Render("Key Identity"),
	}
	identityValueWidth := detailTableValueWidth(contentWidth)
	identityRows := []detailTableRow{
		{Label: "Name", Value: name, Style: theme.HeroTitle},
		{Label: "Type", Value: strings.ToUpper(screen.Key.Type), Style: theme.RowValue},
		{Label: "Fingerprint", Value: screen.Key.Fingerprint, Style: theme.RowValue},
		{
			Label: "Public key",
			Value: compactPublicKeyPreview(screen.Key.PublicKey, max(24, min(40, identityValueWidth))),
			Style: theme.BodyStrong,
		},
		{Label: "Private key", Value: privateKeyVisibilityLabel(), Style: theme.Body},
	}
	sections = append(sections, "", renderDetailTable(identityRows, contentWidth))

	meta := buildMetadata(screen.Key, contentWidth)
	if len(meta) > 0 {
		sections = append(sections,
			"",
			theme.Divider(contentWidth),
			"",
			theme.SectionTitle.Render("Meta Data"),
			"",
			strings.Join(meta, "\n"),
		)
	}

	sections = append(sections, "", renderDetailStatus(screen.Status, screen.StatusError, screen.Busy, spinner))

	return strings.Join(sections, "\n")
}

func renderDetailRow(label, value string) string {
	return padRight(theme.RowLabel.Render(strings.ToUpper(label)), 15) + theme.BodyStrong.Render(value)
}

type detailTableRow struct {
	Label string
	Value string
	Style lipgloss.Style
}

func renderDetailTable(rows []detailTableRow, width int) string {
	labelWidth := detailSharedLabelWidth()
	valueWidth := max(16, width-labelWidth-2)
	lines := make([]string, 0, len(rows)*2)

	for _, row := range rows {
		value := strings.TrimSpace(row.Value)
		if value == "" {
			value = "—"
		}

		label := padRight(theme.RowLabel.Render(strings.ToUpper(row.Label)), labelWidth+2)
		wrapped := wrapDetailText(value, valueWidth)
		if len(wrapped) == 0 {
			wrapped = []string{"—"}
		}

		lines = append(lines, label+row.Style.Render(wrapped[0]))
		for _, line := range wrapped[1:] {
			lines = append(lines, strings.Repeat(" ", labelWidth+2)+row.Style.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func detailTableLabelWidth(rows []detailTableRow) int {
	width := 10
	for _, row := range rows {
		width = max(width, lipgloss.Width(strings.ToUpper(strings.TrimSpace(row.Label))))
	}
	return width
}

func detailSharedLabelWidth() int {
	return detailTableLabelWidth([]detailTableRow{
		{Label: "Name"},
		{Label: "Fingerprint"},
		{Label: "Private key"},
		{Label: "Public key"},
		{Label: "Git signing"},
		{Label: "Last used"},
	})
}

func detailTableValueWidth(width int) int {
	return max(16, width-detailSharedLabelWidth()-2)
}

func renderDetailFieldBlock(label, value string, width int, style lipgloss.Style) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "—"
	}

	lines := []string{theme.RowLabel.Render(strings.ToUpper(label))}
	for _, line := range wrapDetailText(value, width) {
		lines = append(lines, style.Render(line))
	}
	return strings.Join(lines, "\n")
}

func buildMetadata(key actions.KeyDetail, width int) []string {
	rows := make([]detailTableRow, 0, 6)
	if created := strings.TrimSpace(key.CreatedAt); created != "" {
		rows = append(rows, detailTableRow{Label: "Created", Value: humanDateTime(created), Style: theme.Body})
	}
	if updated := strings.TrimSpace(key.UpdatedAt); updated != "" {
		rows = append(rows, detailTableRow{Label: "Updated", Value: humanDateTime(updated), Style: theme.Body})
	}
	if lastUsed := strings.TrimSpace(key.LastUsedAt); lastUsed != "" {
		rows = append(rows, detailTableRow{Label: "Last used", Value: humanDateTime(lastUsed), Style: theme.Body})
	}
	if key.Version > 0 {
		rows = append(rows, detailTableRow{Label: "Version", Value: fmt.Sprintf("%d", key.Version), Style: theme.Body})
	}
	if origin := strings.TrimSpace(key.DeviceOrigin); origin != "" {
		rows = append(rows, detailTableRow{Label: "Device", Value: origin, Style: theme.Body})
	}
	rows = append(rows, detailTableRow{Label: "Git signing", Value: boolLabel(key.GitSigning), Style: theme.Body})

	lines := make([]string, 0, 3)
	if len(rows) > 0 {
		lines = append(lines, renderDetailTable(rows, width))
	}
	if comment := strings.TrimSpace(key.Comment); comment != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, renderDetailFieldBlock("Comment", comment, width, theme.Body))
	}
	return lines
}

func boolLabel(value bool) string {
	if value {
		return "Enabled"
	}
	return "Disabled"
}

func renderDetailStatus(status string, statusError string, busy bool, spinner string) string {
	if strings.TrimSpace(statusError) != "" {
		return theme.Danger.Render("✕ " + displayMessage(statusError))
	}
	if busy && strings.TrimSpace(status) != "" {
		return theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " " + displayMessage(status))
	}
	if strings.TrimSpace(status) != "" {
		return theme.Success.Render("✓ " + displayMessage(status))
	}
	return " "
}

func compactPublicKeyPreview(value string, maxRunes int) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		return compactMiddle(fields[0], maxRunes)
	}

	keyBodyWidth := max(12, maxRunes-len(fields[0])-1)
	preview := fields[0] + " " + compactMiddle(fields[1], keyBodyWidth)
	if len(fields) < 3 {
		return preview
	}
	return preview
}

func compactMiddle(value string, maxRunes int) string {
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 12 {
		return truncateRunes(value, maxRunes)
	}

	head := max(8, (maxRunes-1)/2)
	tail := max(6, maxRunes-head-1)
	if head+tail+1 > maxRunes {
		tail = max(1, maxRunes-head-1)
	}
	if head+tail >= len(runes) {
		return value
	}
	return string(runes[:head]) + "…" + string(runes[len(runes)-tail:])
}

func privateKeyVisibilityLabel() string {
	return "••••••••••••••••"
}

func wrapDetailText(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	if width <= 0 {
		return []string{value}
	}

	paragraphs := strings.Split(value, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		current := ""
		for _, word := range words {
			if current == "" {
				lines, current = appendWrappedToken(lines, word, width)
				continue
			}

			candidate := current + " " + word
			if lipgloss.Width(candidate) <= width {
				current = candidate
				continue
			}

			lines = append(lines, current)
			lines, current = appendWrappedToken(lines, word, width)
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

func appendWrappedToken(lines []string, token string, width int) ([]string, string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return lines, ""
	}
	if lipgloss.Width(token) <= width {
		return lines, token
	}

	remaining := token
	for lipgloss.Width(remaining) > width {
		chunk, rest := splitByDisplayWidth(remaining, width)
		lines = append(lines, chunk)
		remaining = rest
	}
	return lines, remaining
}

func splitByDisplayWidth(value string, width int) (string, string) {
	if width <= 0 {
		return value, ""
	}

	runes := []rune(value)
	currentWidth := 0
	splitIndex := 0
	for index, r := range runes {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width {
			break
		}
		currentWidth += runeWidth
		splitIndex = index + 1
	}

	if splitIndex == 0 {
		return string(runes[:1]), string(runes[1:])
	}
	return string(runes[:splitIndex]), string(runes[splitIndex:])
}

func humanDateTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.In(time.Local).Format("02 Jan 2006, 3:04 PM MST")
}
