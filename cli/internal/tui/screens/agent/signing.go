package agent

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/actions"
	commonscreen "github.com/itzzritik/forged/cli/internal/tui/screens/common"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type SigningKeyRow struct {
	Name        string
	Fingerprint string
	Selected    bool
}

type SigningScreen struct {
	Loading     bool
	Error       string
	Busy        bool
	BusyMessage string
	Status      actions.CommitSigningStatus
	Rows        []SigningKeyRow
}

func RenderSigning(screen SigningScreen, spinner string, width int) string {
	contentWidth := max(28, min(width, theme.HeroMaxWidth+10))
	if screen.Loading && len(screen.Rows) == 0 && strings.TrimSpace(screen.Error) == "" {
		return commonscreen.RenderFullPageLoader(commonscreen.FullPageLoaderScreen{
			Title:       "Opening commit signing",
			Description: "Reading current Git signing state and loading vault keys",
		}, spinner, contentWidth)
	}
	if msg := strings.TrimSpace(screen.Error); msg != "" {
		return theme.Danger.Render("✕ " + msg)
	}

	sections := []string{
		renderStatusCard(screen, spinner, contentWidth),
		"",
		theme.SectionTitle.Render("Available Keys"),
		"",
		renderKeyPicker(screen.Rows, contentWidth),
	}
	return strings.Join(sections, "\n")
}

func renderStatusCard(screen SigningScreen, spinner string, width int) string {
	lines := []string{theme.RowLabel.Render(strings.ToUpper("Commit signing status"))}

	switch screen.Status.Mode {
	case actions.CommitSigningForged:
		lines = append(lines,
			"",
			theme.Success.Render("✓ Forged Signing"),
		)
		if keyName := strings.TrimSpace(screen.Status.KeyName); keyName != "" {
			lines = append(lines,
				theme.BodyStrong.Render("Signing with: "+keyName),
				theme.Body.Render(screen.Status.Fingerprint),
			)
		} else if publicKey := strings.TrimSpace(screen.Status.PublicKey); publicKey != "" {
			lines = append(lines,
				theme.BodyStrong.Render("Signing with Forged"),
				theme.Body.Render(compactSigningValue(publicKey, max(28, width-10))),
			)
		}
	case actions.CommitSigningExternal:
		lines = append(lines,
			"",
			theme.Warning.Render("! External Signing"),
		)
		if publicKey := strings.TrimSpace(screen.Status.PublicKey); publicKey != "" {
			lines = append(lines,
				theme.BodyStrong.Render("Current signing key"),
				theme.Body.Render(compactSigningValue(publicKey, max(28, width-10))),
			)
		}
	default:
		lines = append(lines,
			"",
			theme.Warning.Render("! Commits on this machine are not being signed"),
		)
	}

	if screen.Busy {
		message := strings.TrimSpace(screen.BusyMessage)
		if message == "" {
			message = "Updating signing configuration"
		}
		lines = append(lines, "", theme.BodyStrong.Render(spinner+" "+message))
	}

	card := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.ColorBorder)).
		Padding(1, 2).
		Width(width)

	return card.Render(strings.Join(lines, "\n"))
}

func renderKeyPicker(rows []SigningKeyRow, width int) string {
	if len(rows) == 0 {
		return renderEmptyPicker(width)
	}

	if width < 60 {
		return renderStackedKeyPicker(rows, width)
	}
	return renderTableKeyPicker(rows, width)
}

func renderTableKeyPicker(rows []SigningKeyRow, width int) string {
	nameWidth := max(18, min(30, width/3))
	fpWidth := max(18, width-nameWidth-4)

	lines := []string{
		padRight(theme.RowLabel.Render("NAME"), nameWidth+2) + theme.RowLabel.Render("FINGERPRINT"),
		theme.Divider(width),
	}

	for _, row := range rows {
		prefix := theme.BodyMuted.Render("  ")
		nameStyle := theme.BodyStrong
		fpStyle := theme.Body
		if row.Selected {
			prefix = theme.Bullet.Render("▸ ")
			nameStyle = theme.Kicker
			fpStyle = theme.BodyStrong
		}

		name := truncateRow(row.Name, nameWidth)
		fingerprint := truncateRow(row.Fingerprint, fpWidth)
		lines = append(lines,
			lipgloss.NewStyle().Width(width).Render(
				prefix+
					padRight(nameStyle.Render(name), nameWidth)+
					"  "+
					fpStyle.Render(fingerprint),
			),
		)
	}

	for len(lines) < 10 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderStackedKeyPicker(rows []SigningKeyRow, width int) string {
	lines := []string{}
	for _, row := range rows {
		prefix := theme.BodyMuted.Render("  ")
		nameStyle := theme.BodyStrong
		if row.Selected {
			prefix = theme.Bullet.Render("▸ ")
			nameStyle = theme.Kicker
		}
		lines = append(lines,
			lipgloss.NewStyle().Width(width).Render(prefix+nameStyle.Render(row.Name)),
			lipgloss.NewStyle().Width(width).Render("  "+theme.Body.Render(compactSigningValue(row.Fingerprint, max(22, width-4)))),
		)
	}

	for len(lines) < 10 {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderEmptyPicker(width int) string {
	title := theme.BodyStrong.Render("No keys to show")
	subtitle := theme.BodyMuted.Render("Generate or import a key to enable commit signing")
	lines := make([]string, 8)
	lines[3] = lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
	lines[4] = lipgloss.PlaceHorizontal(width, lipgloss.Center, subtitle)
	return strings.Join(lines, "\n")
}

func truncateRow(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}

	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
}

func compactSigningValue(value string, maxRunes int) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		return compactMiddle(fields[0], maxRunes)
	}

	bodyWidth := max(12, maxRunes-len(fields[0])-1)
	return fields[0] + " " + compactMiddle(fields[1], bodyWidth)
}

func compactMiddle(value string, maxRunes int) string {
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 12 {
		return string(runes[:maxRunes])
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

func padRight(value string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(value)
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
