package doctor

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Tone string

const (
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

type Row struct {
	Check  string
	Status string
	Detail string
	Tone   Tone
}

type Screen struct {
	Rows []Row
}

const compactWidth = 36

func Render(screen Screen, width int) string {
	if len(screen.Rows) == 0 {
		return ""
	}

	tableWidth := max(1, width)
	if tableWidth < compactWidth {
		lines := make([]string, 0, len(screen.Rows)*2)
		for _, row := range screen.Rows {
			lines = append(lines, renderCompactRow(row, tableWidth)...)
		}
		return strings.Join(lines, "\n")
	}

	checkWidth := 14
	statusWidth := 18
	gap := 2
	detailWidth := max(0, tableWidth-checkWidth-statusWidth-gap-gap)

	lines := make([]string, 0, len(screen.Rows))

	for _, row := range screen.Rows {
		lines = append(lines, renderRow(row, checkWidth, statusWidth, detailWidth, gap))
	}

	return strings.Join(lines, "\n")
}

func renderRow(row Row, checkWidth, statusWidth, detailWidth, gap int) string {
	row = sanitizedRow(row)
	check := padRight(theme.BodyStrong.Render(truncateRunes(strings.TrimSpace(row.Check), checkWidth)), checkWidth+gap)
	status := statusStyle(row.Tone).Render(truncateRunes(strings.TrimSpace(row.Status), statusWidth))
	if detailWidth == 0 {
		return padRight(check+status, checkWidth+gap+statusWidth)
	}
	status = padRight(status, statusWidth+gap)
	detail := theme.Body.Render(truncateRunes(strings.TrimSpace(row.Detail), detailWidth))
	return check + status + padRight(detail, detailWidth)
}

func renderCompactRow(row Row, width int) []string {
	row = sanitizedRow(row)
	check := theme.BodyStrong.Render(truncateRunes(strings.TrimSpace(row.Check), width))
	status := statusStyle(row.Tone).Render(truncateRunes(strings.TrimSpace(row.Status), width))
	statusWidth := lipgloss.Width(status)
	detailWidth := max(0, width-statusWidth-2)
	if detailWidth > 0 {
		if detail := truncateRunes(strings.TrimSpace(row.Detail), detailWidth); detail != "" {
			status += "  " + theme.Body.Render(detail)
		}
	}
	return []string{padRight(check, width), padRight(status, width)}
}

func sanitizedRow(row Row) Row {
	row.Check = theme.SanitizeText(row.Check)
	row.Status = theme.SanitizeText(row.Status)
	row.Detail = theme.SanitizeText(row.Detail)
	return row
}

func statusStyle(tone Tone) lipgloss.Style {
	switch tone {
	case ToneSuccess:
		return theme.Success
	case ToneDanger:
		return theme.Danger
	default:
		return theme.Warning
	}
}

func RowHeight(width int) int {
	if width < compactWidth {
		return 2
	}
	return 1
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
	return ansi.Truncate(value, width, theme.Glyphs.Ellipsis)
}
