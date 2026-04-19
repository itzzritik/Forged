package keys

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const visibleRowCount = 8

type BrowserScreen struct {
	SearchView    string
	SearchQuery   string
	SearchActive  bool
	SearchNotice  string
	CountLabel    string
	Rows          []actions.KeySummary
	SelectedIndex int
	Loading       bool
	Error         string
}

func RenderBrowser(screen BrowserScreen, spinner string, width int) string {
	contentWidth := max(36, width)
	sections := []string{}

	if screen.Loading {
		sections = append(sections, theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" Loading keys"))
		return strings.Join(append(sections, "", renderSearchField(screen.SearchView, screen.SearchActive, screen.SearchNotice, screen.CountLabel, contentWidth)), "\n")
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		sections = append(sections, theme.Danger.Render("✕ "+displayMessage(msg)))
		return strings.Join(append(sections, "", renderSearchField(screen.SearchView, screen.SearchActive, screen.SearchNotice, screen.CountLabel, contentWidth)), "\n")
	}

	sections = append(sections, renderBrowserTable(screen, contentWidth))
	sections = append(sections, "", renderSearchField(screen.SearchView, screen.SearchActive, screen.SearchNotice, screen.CountLabel, contentWidth))
	return strings.Join(sections, "\n")
}

func VisibleRows() int {
	return visibleRowCount
}

func renderSearchField(view string, active bool, notice string, countLabel string, width int) string {
	value := strings.TrimRight(view, " ")
	if value == "" {
		value = theme.BodyMuted.Render("Search keys")
	} else if active {
		value = theme.FieldValue.Render(value)
	} else {
		value = theme.Body.Render(value)
	}

	fieldWidth := max(20, width+shell.ContentLeftInset+shell.ContentRightInset)
	lines := []string{renderBrowserMetaRow(notice, width)}
	lines = append(lines,
		shell.FullBleed(theme.Divider(fieldWidth)),
		shell.JoinRow(width, theme.Kicker.Render("❯")+"  "+value, renderBrowserCountLabel(countLabel)),
	)
	return strings.Join(lines, "\n")
}

func renderBrowserMetaRow(notice string, width int) string {
	left := ""
	if msg := strings.TrimSpace(displayMessage(notice)); msg != "" {
		left = theme.Warning.Render(truncateRunes(msg, width))
	}

	row := shell.JoinRow(width, left, "")
	if strings.TrimSpace(row) == "" {
		return " "
	}
	return row
}

func renderBrowserCountLabel(countLabel string) string {
	if strings.TrimSpace(countLabel) == "" {
		return ""
	}
	return theme.BodyMuted.Render(countLabel)
}

func renderBrowserTable(screen BrowserScreen, width int) string {
	tableWidth := max(44, width)
	selectionWidth := 2
	nameWidth := 28
	typeWidth := 10
	columnGap := 2
	minFingerprintWidth := 16
	requiredWidth := selectionWidth + nameWidth + typeWidth + minFingerprintWidth + columnGap + columnGap
	if requiredWidth > tableWidth {
		nameWidth = max(18, tableWidth-selectionWidth-typeWidth-minFingerprintWidth-columnGap-columnGap)
	}
	fingerprintWidth := max(minFingerprintWidth, tableWidth-selectionWidth-nameWidth-typeWidth-columnGap-columnGap)

	lines := []string{
		renderBrowserHeader(selectionWidth, nameWidth, typeWidth, fingerprintWidth, columnGap),
		shell.FullBleed(theme.Divider(tableWidth + shell.ContentLeftInset + shell.ContentRightInset)),
		"",
	}

	if len(screen.Rows) == 0 {
		lines = append(lines, renderBrowserEmptyRows(tableWidth, browserEmptySubtitle(screen.SearchQuery))...)
		return strings.Join(lines, "\n")
	}

	for index := 0; index < visibleRowCount; index++ {
		if index >= len(screen.Rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, renderBrowserRow(screen.Rows[index], index == screen.SelectedIndex, selectionWidth, nameWidth, typeWidth, fingerprintWidth, columnGap))
	}

	return strings.Join(lines, "\n")
}

func renderBrowserHeader(selectionWidth, nameWidth, typeWidth, fingerprintWidth, columnGap int) string {
	selection := strings.Repeat(" ", selectionWidth)
	name := padRight(theme.RowLabel.Render("NAME"), nameWidth+columnGap)
	keyType := padRight(theme.RowLabel.Render("TYPE"), typeWidth+columnGap)
	fingerprint := theme.RowLabel.Render(truncateRunes("FINGERPRINT", fingerprintWidth))
	return selection + name + keyType + fingerprint
}

func renderBrowserRow(key actions.KeySummary, selected bool, selectionWidth, nameWidth, typeWidth, fingerprintWidth, columnGap int) string {
	prefix := strings.Repeat(" ", selectionWidth)
	nameStyle := theme.FieldValue
	detailStyle := theme.BodyMuted
	if selected {
		prefix = theme.Kicker.Render("▸")
		nameStyle = theme.Kicker
		detailStyle = theme.Body
	}

	name := truncateRunes(key.Name, nameWidth)
	keyType := truncateRunes(strings.ToUpper(key.Type), typeWidth)
	fingerprint := truncateRunes(key.Fingerprint, fingerprintWidth)

	return padRight(prefix, selectionWidth) +
		padRight(nameStyle.Render(name), nameWidth+columnGap) +
		padRight(detailStyle.Render(keyType), typeWidth+columnGap) +
		detailStyle.Render(fingerprint)
}

func renderBrowserEmptyRows(width int, subtitle string) []string {
	rows := make([]string, visibleRowCount)
	titleRow := max(0, visibleRowCount/2-1)
	subtitleRow := min(visibleRowCount-1, titleRow+1)

	rows[titleRow] = centerRow(theme.BodyStrong.Render("No keys to show"), width)
	if msg := strings.TrimSpace(subtitle); msg != "" {
		rows[subtitleRow] = centerRow(theme.BodyMuted.Render(msg), width)
	}
	return rows
}

func browserEmptySubtitle(query string) string {
	if strings.TrimSpace(query) != "" {
		return "Try a different search term"
	}
	return "Generate or import a key to get started"
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

func centerRow(value string, width int) string {
	visible := lipgloss.Width(value)
	if visible >= width {
		return value
	}

	leftPad := max(0, (width-visible)/2)
	rightPad := max(0, width-leftPad-visible)
	return strings.Repeat(" ", leftPad) + value + strings.Repeat(" ", rightPad)
}
