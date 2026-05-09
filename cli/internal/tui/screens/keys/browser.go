package keys

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

const visibleRowCount = 8

type BrowserRow struct {
	Name        string
	Type        string
	Fingerprint string
	StatusIcon  string
}

type BrowserScreen struct {
	SearchView       string
	SearchQuery      string
	SearchActive     bool
	SearchNotice     string
	CountLabel       string
	NameHeader       string
	TypeHeader       string
	DetailHeader     string
	NameWidth        int
	TypeWidth        int
	MinDetailWidth   int
	VisibleRows      int
	PreserveTypeCase bool
	Rows             []BrowserRow
	SelectedIndex    int
	Loading          bool
	Error            string
	ShowTopBorder    bool
	ShowStatus       bool
	HideFooter       bool
	EmptyTitle       string
	EmptySubtitle    string
}

func RenderBrowser(screen BrowserScreen, spinner string, width int) string {
	contentWidth := max(36, width)
	searchField := renderSearchField(screen.SearchView, screen.SearchActive, screen.SearchNotice, screen.CountLabel, contentWidth)

	if screen.Loading {
		top := strings.Join([]string{
			theme.BodyStrong.Render(theme.Spinner.Render(spinner) + " Loading keys"),
			"",
		}, "\n")
		if screen.HideFooter {
			return strings.TrimRight(top, "\n")
		}
		return shell.DockBottom(top, searchField)
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		top := strings.Join([]string{
			theme.Danger.Render("✕ " + displayMessage(msg)),
			"",
		}, "\n")
		if screen.HideFooter {
			return strings.TrimRight(top, "\n")
		}
		return shell.DockBottom(top, searchField)
	}

	top := strings.Join([]string{
		renderBrowserTable(screen, contentWidth),
		"",
	}, "\n")
	if screen.HideFooter {
		return strings.TrimRight(top, "\n")
	}
	return shell.DockBottom(top, searchField)
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
	visibleRows := browserVisibleRows(screen)
	selectionWidth := 2
	nameWidth := screen.NameWidth
	if nameWidth <= 0 {
		nameWidth = 28
	}
	typeWidth := screen.TypeWidth
	if typeWidth <= 0 {
		typeWidth = 10
	}
	columnGap := 2
	statusWidth := 0
	if screen.ShowStatus {
		statusWidth = 2
	}
	minFingerprintWidth := screen.MinDetailWidth
	if minFingerprintWidth <= 0 {
		minFingerprintWidth = 16
	}
	requiredWidth := selectionWidth + nameWidth + typeWidth + minFingerprintWidth + columnGap + columnGap
	if screen.ShowStatus {
		requiredWidth += columnGap + statusWidth
	}
	if requiredWidth > tableWidth {
		nameWidth = max(18, tableWidth-selectionWidth-typeWidth-minFingerprintWidth-columnGap-columnGap-statusWidth)
		if screen.ShowStatus {
			nameWidth = max(18, nameWidth-columnGap)
		}
	}
	fingerprintWidth := max(minFingerprintWidth, tableWidth-selectionWidth-nameWidth-typeWidth-columnGap-columnGap)
	if screen.ShowStatus {
		fingerprintWidth = max(minFingerprintWidth, fingerprintWidth-columnGap-statusWidth)
	}

	lines := []string{}
	if screen.ShowTopBorder {
		lines = append(lines, renderBrowserDivider(tableWidth))
	}
	nameHeader := firstNonEmpty(screen.NameHeader, "NAME")
	typeHeader := firstNonEmpty(screen.TypeHeader, "TYPE")
	detailHeader := firstNonEmpty(screen.DetailHeader, "FINGERPRINT")
	lines = append(lines,
		renderBrowserHeader(nameHeader, typeHeader, detailHeader, selectionWidth, nameWidth, typeWidth, fingerprintWidth, statusWidth, columnGap),
		renderBrowserDivider(tableWidth),
		"",
	)

	if len(screen.Rows) == 0 {
		lines = append(lines, renderBrowserEmptyRows(tableWidth, visibleRows, browserEmptyTitle(screen.EmptyTitle), browserEmptySubtitle(screen.SearchQuery, screen.EmptySubtitle))...)
		return strings.Join(lines, "\n")
	}

	for index := 0; index < visibleRows; index++ {
		if index >= len(screen.Rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, renderBrowserRow(screen.Rows[index], index == screen.SelectedIndex, screen.PreserveTypeCase, selectionWidth, nameWidth, typeWidth, fingerprintWidth, statusWidth, columnGap))
	}

	return strings.Join(lines, "\n")
}

func renderBrowserDivider(tableWidth int) string {
	return shell.FullBleed(theme.Divider(tableWidth + shell.ContentLeftInset + shell.ContentRightInset))
}

func renderBrowserHeader(nameHeader, typeHeader, detailHeader string, selectionWidth, nameWidth, typeWidth, fingerprintWidth, statusWidth, columnGap int) string {
	selection := strings.Repeat(" ", selectionWidth)
	name := padRight(theme.RowLabel.Render(truncateRunes(nameHeader, nameWidth)), nameWidth+columnGap)
	keyType := padRight(theme.RowLabel.Render(truncateRunes(typeHeader, typeWidth)), typeWidth+columnGap)
	fingerprint := theme.RowLabel.Render(truncateRunes(detailHeader, fingerprintWidth))
	if statusWidth == 0 {
		return selection + name + keyType + fingerprint
	}
	return selection + name + keyType + padRight(fingerprint, fingerprintWidth+columnGap) + padRight("", statusWidth)
}

func renderBrowserRow(key BrowserRow, selected bool, preserveTypeCase bool, selectionWidth, nameWidth, typeWidth, fingerprintWidth, statusWidth, columnGap int) string {
	prefix := strings.Repeat(" ", selectionWidth)
	nameStyle := theme.FieldValue
	detailStyle := theme.BodyMuted
	if selected {
		prefix = theme.Kicker.Render("▸")
		nameStyle = theme.Kicker
		detailStyle = theme.Body
	}

	name := truncateRunes(key.Name, nameWidth)
	keyTypeValue := key.Type
	if !preserveTypeCase {
		keyTypeValue = strings.ToUpper(keyTypeValue)
	}
	keyType := truncateRunes(keyTypeValue, typeWidth)
	fingerprint := truncateRunes(key.Fingerprint, fingerprintWidth)

	row := padRight(prefix, selectionWidth) +
		padRight(nameStyle.Render(name), nameWidth+columnGap) +
		padRight(detailStyle.Render(keyType), typeWidth+columnGap) +
		detailStyle.Render(fingerprint)
	if statusWidth == 0 {
		return row
	}

	status := ""
	if icon := strings.TrimSpace(key.StatusIcon); icon != "" {
		status = theme.Success.Render(icon)
	}
	return row + strings.Repeat(" ", columnGap) + padRight(status, statusWidth)
}

func browserVisibleRows(screen BrowserScreen) int {
	if screen.VisibleRows > 0 {
		return screen.VisibleRows
	}
	return visibleRowCount
}

func renderBrowserEmptyRows(width int, visibleRows int, title string, subtitle string) []string {
	rows := make([]string, visibleRows)
	titleRow := max(0, visibleRows/2-1)
	subtitleRow := min(visibleRows-1, titleRow+1)

	rows[titleRow] = centerRow(theme.BodyStrong.Render(title), width)
	if msg := strings.TrimSpace(subtitle); msg != "" {
		rows[subtitleRow] = centerRow(theme.BodyMuted.Render(msg), width)
	}
	return rows
}

func browserEmptyTitle(fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "No keys to show"
}

func browserEmptySubtitle(query string, fallback string) string {
	if strings.TrimSpace(query) != "" {
		return "Try a different search term"
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "Generate or import a key to get started"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
