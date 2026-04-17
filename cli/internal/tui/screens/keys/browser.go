package keys

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const visibleRowCount = 5

type BrowserScreen struct {
	SearchView    string
	SearchActive  bool
	SearchNotice  string
	Rows          []actions.KeySummary
	SelectedIndex int
	Loading       bool
	Error         string
}

func RenderBrowser(screen BrowserScreen, spinner string, width int) string {
	contentWidth := max(28, width)
	sections := []string{
		renderSearchField(screen.SearchView, screen.SearchActive, contentWidth),
	}

	if screen.Loading {
		sections = append(sections, "", theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" Loading keys"))
		return strings.Join(sections, "\n")
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		sections = append(sections, "", theme.Danger.Render("✕ "+msg))
		return strings.Join(sections, "\n")
	}

	if notice := strings.TrimSpace(screen.SearchNotice); notice != "" {
		sections = append(sections, "", theme.BodyMuted.Width(min(contentWidth, theme.HeroMaxWidth)).Render(notice))
	}

	if len(screen.Rows) == 0 {
		sections = append(sections, "", theme.BodyMuted.Render("No keys found"))
		return strings.Join(sections, "\n")
	}

	sections = append(sections, "", renderBrowserTable(screen, contentWidth))
	return strings.Join(sections, "\n")
}

func VisibleRows() int {
	return visibleRowCount
}

func renderSearchField(view string, active bool, width int) string {
	lineStyle := theme.FieldLineIdle
	if active {
		lineStyle = theme.FieldLineActive
	}

	value := strings.TrimSpace(view)
	if value == "" {
		value = theme.BodyMuted.Render("Search keys")
	}

	fieldWidth := max(20, min(width, theme.HeroMaxWidth))
	lines := []string{
		theme.FieldLabel.Render("Search"),
		value,
		lineStyle.Render(strings.Repeat("─", fieldWidth)),
	}
	return strings.Join(lines, "\n")
}

func renderBrowserTable(screen BrowserScreen, width int) string {
	listWidth := max(36, min(width, theme.HeroMaxWidth+8))
	nameWidth := max(14, listWidth-27)
	typeWidth := 8
	fingerprintWidth := max(10, listWidth-nameWidth-typeWidth-7)

	lines := []string{
		renderBrowserHeader(nameWidth, typeWidth, fingerprintWidth),
	}

	for index := 0; index < visibleRowCount; index++ {
		if index >= len(screen.Rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, renderBrowserRow(screen.Rows[index], index == screen.SelectedIndex, nameWidth, typeWidth, fingerprintWidth))
	}

	return strings.Join(lines, "\n")
}

func renderBrowserHeader(nameWidth, typeWidth, fingerprintWidth int) string {
	left := padRight(theme.RowLabel.Render("NAME"), nameWidth+2)
	keyType := padRight(theme.RowLabel.Render("TYPE"), typeWidth+1)
	fingerprint := truncateRunes("FINGERPRINT", fingerprintWidth)
	return left + keyType + theme.RowLabel.Render(fingerprint)
}

func renderBrowserRow(key actions.KeySummary, selected bool, nameWidth, typeWidth, fingerprintWidth int) string {
	prefix := theme.BodyMuted.Render(" ")
	nameStyle := theme.Body
	if selected {
		prefix = theme.Kicker.Render("›")
		nameStyle = theme.Kicker
	}

	name := truncateRunes(key.Name, nameWidth)
	keyType := truncateRunes(strings.ToUpper(key.Type), typeWidth)
	fingerprint := truncateRunes(key.Fingerprint, fingerprintWidth)

	return prefix + " " +
		padRight(nameStyle.Render(name), nameWidth+1) +
		padRight(theme.BodyMuted.Render(keyType), typeWidth+1) +
		theme.BodyMuted.Render(fingerprint)
}

func padRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	visible := lipgloss.Width(value)
	if visible >= width {
		return value
	}
	return value + strings.Repeat(" ", width-visible)
}

func truncateRunes(value string, width int) string {
	if width <= 0 || utf8.RuneCountInString(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}
