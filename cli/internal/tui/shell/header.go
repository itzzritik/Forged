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
	Tone  StatusTone
}

type HeaderData struct {
	PageTitle   string
	Breadcrumbs []Breadcrumb
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
	if titleRow := renderTitleRow(width, data.PageTitle, data.Breadcrumbs); titleRow != "" {
		lines = append(lines, "", titleRow, "", theme.Divider(width))
	}
	return strings.Join(lines, "\n")
}

func renderHeaderBox(width int, data HeaderData) string {
	innerWidth := max(48, width-4)
	sidebarWidth := min(32, max(28, innerWidth/3))
	leftWidth := max(18, innerWidth-sidebarWidth-1)

	logo := renderBrandBanner(leftWidth)
	sidebar := renderSidebar(data.Version, data.StatusItems, sidebarWidth)

	var content string
	if innerWidth >= 68 {
		leftBlock := lipgloss.NewStyle().Width(leftWidth).Render(logo)
		rightBlock := theme.HeaderSidebar.Width(sidebarWidth).Render(sidebar)
		content = lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, " ", rightBlock)
	} else {
		content = strings.Join([]string{
			logo,
			"",
			theme.HeaderSidebar.Render(sidebar),
		}, "\n")
	}

	return theme.HeaderFrame.Width(max(44, innerWidth)).Render(content)
}

func renderBrandBanner(width int) string {
	if width > 0 && width < lipgloss.Width(brandBanner) {
		return theme.Kicker.Render("FORGED")
	}
	return theme.BrandBanner.Render(brandBanner)
}

func renderSidebar(version string, items []StatusItem, width int) string {
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

func renderStatusItem(item StatusItem) string {
	icon := theme.Danger.Render("✕")
	switch item.Tone {
	case StatusToneSuccess:
		icon = theme.Success.Render("✓")
	case StatusToneWarning:
		icon = theme.Warning.Render("!")
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
			parts = append(parts, theme.BreadcrumbSeparator.Render("›"))
		}
		if item.Current {
			parts = append(parts, theme.BreadcrumbCurrent.Render(strings.ToUpper(item.Label)))
			continue
		}
		parts = append(parts, theme.Breadcrumb.Render(strings.ToUpper(item.Label)))
	}
	return strings.Join(parts, " ")
}

func renderTitleRow(width int, title string, breadcrumbs []Breadcrumb) string {
	left := ""
	if strings.TrimSpace(title) != "" {
		left = theme.SectionTitle.Render(strings.ToUpper(title))
	}
	right := renderBreadcrumbs(breadcrumbs)
	if left == "" && right == "" {
		return ""
	}
	if left != "" && right != "" && lipgloss.Width(left)+lipgloss.Width(right)+4 > width {
		return left + "\n" + right
	}
	return JoinRow(width, left, right)
}
