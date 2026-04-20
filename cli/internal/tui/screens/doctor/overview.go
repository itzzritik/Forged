package doctor

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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

func Render(screen Screen, width int) string {
	if len(screen.Rows) == 0 {
		return ""
	}

	tableWidth := max(56, width)
	checkWidth := 14
	statusWidth := 18
	gap := 2
	detailWidth := max(12, tableWidth-checkWidth-statusWidth-gap-gap)

	lines := make([]string, 0, len(screen.Rows))

	for _, row := range screen.Rows {
		lines = append(lines, renderRow(row, checkWidth, statusWidth, detailWidth, gap))
	}

	return strings.Join(lines, "\n")
}

func renderRow(row Row, checkWidth, statusWidth, detailWidth, gap int) string {
	statusStyle := theme.Warning
	switch row.Tone {
	case ToneSuccess:
		statusStyle = theme.Success
	case ToneDanger:
		statusStyle = theme.Danger
	}

	check := padRight(theme.BodyStrong.Render(truncateRunes(strings.TrimSpace(row.Check), checkWidth)), checkWidth+gap)
	status := padRight(statusStyle.Render(truncateRunes(strings.TrimSpace(row.Status), statusWidth)), statusWidth+gap)
	detail := theme.Body.Render(truncateRunes(strings.TrimSpace(row.Detail), detailWidth))
	return check + status + detail
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
	if width <= 0 || len([]rune(value)) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
