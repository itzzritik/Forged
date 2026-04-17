package shell

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Breadcrumb struct {
	Label   string
	Current bool
}

type StatusTone string

const (
	StatusToneSuccess StatusTone = "success"
	StatusToneWarning StatusTone = "warning"
	StatusToneDanger  StatusTone = "danger"
)

type StatusItem struct {
	Label string
	Icon  string
	Tone  StatusTone
}

type HeaderData struct {
	PageTitle   string
	Breadcrumbs []Breadcrumb
	PageNote    string
	Version     string
	StatusItems []StatusItem
}

const brandBanner = `▄▄▄▄▄▄▄   ▄▄▄▄▄   ▄▄▄▄▄▄▄    ▄▄▄▄▄▄▄   ▄▄▄▄▄▄▄ ▄▄▄▄▄▄
███▀▀▀▀▀ ▄███████▄ ███▀▀███▄ ███▀▀▀▀▀  ███▀▀▀▀▀ ███▀▀██▄
███▄▄    ███   ███ ███▄▄███▀ ███       ███▄▄    ███  ███
███▀▀    ███▄▄▄███ ███▀▀██▄  ███  ███▀ ███      ███  ███
███       ▀█████▀  ███  ▀███ ▀██████▀  ▀███████ ██████▀`

func RenderHeader(width int, data HeaderData) string {
	lines := []string{renderHeaderBox(width, data)}
	if titleRow := renderTitleRow(width, data.PageTitle, data.Breadcrumbs, data.PageNote); titleRow != "" {
		lines = append(lines, "", titleRow, "", theme.Divider(width))
	}
	return strings.Join(lines, "\n")
}

func renderHeaderBox(width int, data HeaderData) string {
	innerWidth := max(16, width-4)
	sidebar := renderSidebar(data.Version, data.StatusItems)
	sidebarBlock := theme.HeaderSidebar.Render(sidebar)
	sidebarWidth := min(max(lipgloss.Width(sidebarBlock), 28), max(28, innerWidth-20))
	leftLimit := max(18, innerWidth-sidebarWidth-3)

	logo := renderBrandBanner(leftLimit)

	var content string
	if innerWidth >= 68 {
		leftBlock := lipgloss.NewStyle().Width(lipgloss.Width(logo)).Render(logo)
		rightBlock := theme.HeaderSidebar.Width(sidebarWidth).Render(sidebar)
		height := max(lipgloss.Height(leftBlock), lipgloss.Height(rightBlock))
		separator := renderHeaderSeparator(height)

		leftBlock = padBlockHeight(leftBlock, height)
		rightBlock = padBlockHeight(rightBlock, height)
		leftGap, rightGap := balancedHeaderGaps(innerWidth, lipgloss.Width(leftBlock), lipgloss.Width(separator), lipgloss.Width(rightBlock))

		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftBlock,
			strings.Repeat(" ", leftGap),
			separator,
			strings.Repeat(" ", rightGap),
			rightBlock,
		)
	} else {
		content = strings.Join([]string{
			logo,
			"",
			theme.HeaderSidebar.Render(sidebar),
		}, "\n")
	}

	return theme.HeaderFrame.Width(innerWidth).Render(content)
}

func renderBrandBanner(width int) string {
	if width > 0 && width < lipgloss.Width(brandBanner) {
		return theme.Kicker.Render("FORGED")
	}
	return theme.BrandBanner.Render(brandBanner)
}

func renderSidebar(version string, items []StatusItem) string {
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}

	versionLine := theme.HeaderVersionLabel.Render("VERSION") + " " + theme.HeaderVersionValue.Render("v"+version)
	lines := []string{versionLine}

	if len(items) > 0 {
		lines = append(lines, "")
	}

	for _, item := range items {
		lines = append(lines, renderStatusItem(item))
	}

	return strings.Join(lines, "\n")
}

func renderHeaderSeparator(height int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, height)
	for index := range lines {
		lines[index] = theme.HeaderSeparator.Render("│")
	}
	return strings.Join(lines, "\n")
}

func balancedHeaderGaps(totalWidth int, leftWidth int, separatorWidth int, rightWidth int) (int, int) {
	remaining := totalWidth - leftWidth - separatorWidth - rightWidth
	if remaining <= 0 {
		return 0, 0
	}
	leftGap := remaining / 2
	rightGap := remaining - leftGap
	bias := min(2, rightGap)
	leftGap += bias
	rightGap -= bias
	return leftGap, rightGap
}

func padBlockHeight(block string, targetHeight int) string {
	lines := strings.Split(block, "\n")
	width := lipgloss.Width(block)
	for len(lines) < targetHeight {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func renderStatusItem(item StatusItem) string {
	icon := theme.Danger.Render("✕")
	if strings.TrimSpace(item.Icon) != "" {
		icon = theme.Kicker.Render(item.Icon)
	} else {
		switch item.Tone {
		case StatusToneSuccess:
			icon = theme.Success.Render("✓")
		case StatusToneWarning:
			icon = theme.Warning.Render("!")
		}
	}
	text := theme.BodyStrong.Render(item.Label)
	return icon + " " + text
}

func renderBreadcrumbs(items []Breadcrumb) string {
	if len(items) < 2 {
		return ""
	}

	parts := make([]string, 0, len(items)*2)
	for index, item := range items {
		if index > 0 {
			parts = append(parts, theme.BreadcrumbSeparator.Render("❱"))
		}
		if item.Current {
			parts = append(parts, theme.BreadcrumbCurrent.Render(strings.ToUpper(item.Label)))
			continue
		}
		parts = append(parts, theme.Breadcrumb.Render(strings.ToUpper(item.Label)))
	}
	return strings.Join(parts, " ")
}

func renderTitleNote(note string) string {
	if strings.TrimSpace(note) == "" {
		return ""
	}
	return theme.BodyMuted.Render(note)
}

func renderTitleRow(width int, title string, breadcrumbs []Breadcrumb, note string) string {
	left := ""
	if strings.TrimSpace(title) != "" {
		left = theme.SectionTitle.Render(strings.ToUpper(title))
	}
	right := renderBreadcrumbs(breadcrumbs)
	if right == "" {
		right = renderTitleNote(note)
	}
	if left == "" && right == "" {
		return ""
	}
	innerWidth := max(0, width-ContentLeftInset-ContentRightInset)
	row := ""
	if left != "" && right != "" && lipgloss.Width(left)+lipgloss.Width(right)+4 > innerWidth {
		row = left + "\n" + right
	} else {
		row = JoinRow(innerWidth, left, right)
	}
	lines := strings.Split(row, "\n")
	for index, line := range lines {
		lines[index] = strings.Repeat(" ", ContentLeftInset) + line + strings.Repeat(" ", ContentRightInset)
	}
	return strings.Join(lines, "\n")
}
