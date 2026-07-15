package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/components"
	"github.com/itzzritik/forged/cli/internal/tui/shell"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Tone string

const (
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

type Notice struct {
	Message string
	Tone    Tone
}

type Option struct {
	Label       string
	Description string
	Primary     bool
	Selected    bool
}

type Tab struct {
	Label    string
	Selected bool
}

type Page struct {
	Label    string
	Selected bool
}

type Screen struct {
	Title      string
	Context    string
	Notice     Notice
	Options    []Option
	Tabs       []Tab
	Pages      []Page
	Summary    string
	BodyHeight int
}

const (
	stackedLeftInset  = 2
	stackedRightInset = 4
	tabGap            = 1
	tabPageGap        = 2
	pageListMinHeight = 6
	flexTabMinWidth   = 10
)

func Render(screen Screen, width int) string {
	if len(screen.Options) > 0 {
		return renderWelcome(screen, width)
	}
	if len(screen.Tabs) > 0 {
		return renderTabbedDashboard(screen, width)
	}
	sections := make([]string, 0, 2)
	if notice := renderNotice(screen.Notice, width); notice != "" {
		sections = append(sections, notice)
	}
	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(1, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}
	return strings.Join(sections, "\n")
}

func renderTabbedDashboard(screen Screen, width int) string {
	tabWidth := max(16, width)
	notice := renderNotice(screen.Notice, width)
	tabBar := renderTabs(screen.Tabs, tabWidth)
	bottom := ""
	if strings.TrimSpace(screen.Summary) != "" {
		bottom = theme.BodyMuted.Width(max(1, min(width, theme.HeroMaxWidth))).Render(screen.Summary)
	}
	prefix := renderTabbedPrefix(notice, tabBar)
	pageGap := tabPageGap
	pageHeight := 0
	dockGap := 1
	if len(screen.Pages) > 0 && screen.BodyHeight > 0 {
		pageHeight = screen.BodyHeight - lipgloss.Height(prefix) - pageGap
		if bottom != "" {
			pageHeight -= lipgloss.Height(bottom) + dockGap
		}
		if pageHeight < 1 && pageGap > 0 {
			pageGap = 0
			pageHeight = screen.BodyHeight - lipgloss.Height(prefix)
			if bottom != "" {
				pageHeight -= lipgloss.Height(bottom) + dockGap
			}
		}
		if pageHeight < 1 && bottom != "" && dockGap > 0 {
			dockGap = 0
			pageHeight = screen.BodyHeight - lipgloss.Height(prefix) - pageGap - lipgloss.Height(bottom)
		}
		if pageHeight < 1 && bottom != "" {
			bottom = ""
			pageHeight = screen.BodyHeight - lipgloss.Height(prefix) - pageGap
		}
		if pageHeight < 1 && notice != "" {
			notice = compactDashboardNotice(notice)
			prefix = renderTabbedPrefix(notice, tabBar)
			pageHeight = screen.BodyHeight - lipgloss.Height(prefix) - pageGap
		}
		if pageHeight < 1 && tabBar != "" {
			tabBar = ""
			prefix = renderTabbedPrefix(notice, tabBar)
			pageGap = 0
			pageHeight = screen.BodyHeight - lipgloss.Height(prefix)
		}
		if pageHeight < 1 {
			prefix = ""
			pageGap = 0
			pageHeight = screen.BodyHeight
		}
	}

	top := prefix
	if len(screen.Pages) > 0 {
		pages := renderPages(screen.Pages, width, pageHeight)
		if top != "" && pages != "" {
			top += strings.Repeat("\n", pageGap+1)
		}
		top += pages
	}
	if bottom == "" {
		return top
	}
	if strings.TrimSpace(top) != "" && dockGap > 0 {
		top += "\n"
	}
	return shell.DockBottom(top, bottom)
}

func renderTabbedPrefix(notice string, tabBar string) string {
	sections := make([]string, 0, 3)
	if notice != "" {
		sections = append(sections, notice)
		if tabBar != "" {
			sections = append(sections, "")
		}
	}
	if tabBar != "" {
		sections = append(sections, tabBar)
	}
	return strings.Join(sections, "\n")
}

func compactDashboardNotice(notice string) string {
	line, _, _ := strings.Cut(notice, "\n")
	return line
}

func renderTabs(tabs []Tab, width int) string {
	if len(tabs) == 0 {
		return ""
	}

	available := max(16, width)
	minFlexTotal := len(tabs)*flexTabMinWidth + max(0, len(tabs)-1)*tabGap
	if available >= minFlexTotal {
		return renderConnectedTabs(tabs, available)
	}

	selected := 0
	rendered := make([]string, 0, len(tabs))
	widths := make([]int, 0, len(tabs))
	totalWidth := 0
	for index, tab := range tabs {
		if tab.Selected {
			selected = index
		}
		block := renderTab(tab)
		rendered = append(rendered, block)
		blockWidth := lipgloss.Width(block)
		widths = append(widths, blockWidth)
		totalWidth += blockWidth
		if index > 0 {
			totalWidth += tabGap
		}
	}

	if totalWidth <= available {
		return joinTabBlocks(rendered)
	}

	start := selected
	end := selected
	currentWidth := widths[selected]
	for {
		expanded := false
		if start > 0 {
			nextWidth := currentWidth + tabGap + widths[start-1]
			if nextWidth <= available {
				start--
				currentWidth = nextWidth
				expanded = true
			}
		}
		if end < len(rendered)-1 {
			nextWidth := currentWidth + tabGap + widths[end+1]
			if nextWidth <= available {
				end++
				currentWidth = nextWidth
				expanded = true
			}
		}
		if !expanded {
			break
		}
	}

	visible := rendered[start : end+1]
	leftOverflow := start > 0
	rightOverflow := end < len(rendered)-1
	overflowWidth := 0
	if leftOverflow {
		overflowWidth += 2
	}
	if rightOverflow {
		overflowWidth += 2
	}

	for currentWidth+overflowWidth > available && len(visible) > 1 {
		if end-selected > selected-start {
			currentWidth -= widths[end] + tabGap
			end--
		} else {
			currentWidth -= widths[start] + tabGap
			start++
		}
		visible = rendered[start : end+1]
		leftOverflow = start > 0
		rightOverflow = end < len(rendered)-1
		overflowWidth = 0
		if leftOverflow {
			overflowWidth += 2
		}
		if rightOverflow {
			overflowWidth += 2
		}
	}

	row := joinTabBlocks(visible)
	if leftOverflow {
		row = theme.BodyMuted.Render(theme.Glyphs.Ellipsis+" ") + row
	}
	if rightOverflow {
		row += theme.BodyMuted.Render(" " + theme.Glyphs.Ellipsis)
	}
	return row
}

func renderConnectedTabs(tabs []Tab, width int) string {
	innerWidth := max(len(tabs)*flexTabMinWidth, width-max(0, len(tabs)-1)*tabGap)
	baseWidth := innerWidth / len(tabs)
	remainder := innerWidth % len(tabs)

	blocks := make([]string, 0, len(tabs))
	for index, tab := range tabs {
		segmentWidth := baseWidth
		if index < remainder {
			segmentWidth++
		}
		blocks = append(blocks, renderTab(tab, segmentWidth))
	}
	return joinTabBlocks(blocks)
}

func renderTab(tab Tab, width ...int) string {
	labelStyle := theme.BodyStrong
	borderStyle := theme.HeaderSeparator

	if tab.Selected {
		labelStyle = theme.Kicker
		borderStyle = theme.Kicker
	}

	totalWidth := 0
	if len(width) > 0 {
		totalWidth = max(4, width[0])
	}

	bodyWidth := max(1, totalWidth-2)
	body := lipgloss.NewStyle().Align(lipgloss.Center)
	if totalWidth > 0 {
		body = body.Width(bodyWidth)
	} else {
		body = body.Padding(0, 1)
		bodyWidth = lipgloss.Width(body.Render(labelStyle.Render(tab.Label)))
	}

	top := borderStyle.Render(theme.Glyphs.TopLeft) + borderStyle.Render(strings.Repeat(theme.Glyphs.Horizontal, bodyWidth)) + borderStyle.Render(theme.Glyphs.TopRight)
	middle := borderStyle.Render(theme.Glyphs.Vertical) + body.Render(labelStyle.Render(tab.Label)) + borderStyle.Render(theme.Glyphs.Vertical)
	bottom := borderStyle.Render(theme.Glyphs.BottomLeft) + borderStyle.Render(strings.Repeat(theme.Glyphs.Horizontal, bodyWidth)) + borderStyle.Render(theme.Glyphs.BottomRight)
	return strings.Join([]string{top, middle, bottom}, "\n")
}

func joinTabBlocks(blocks []string) string {
	parts := make([]string, 0, len(blocks)*2)
	for index, block := range blocks {
		if index > 0 {
			parts = append(parts, strings.Repeat(" ", tabGap))
		}
		parts = append(parts, block)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func renderPages(pages []Page, width int, maxHeight int) string {
	if len(pages) == 0 {
		return ""
	}

	items := make([]components.SelectionListItem, 0, len(pages))
	for _, page := range pages {
		items = append(items, components.SelectionListItem{
			Label:    page.Label,
			Selected: page.Selected,
		})
	}
	return components.RenderSelectionListPage(items, width, pageListMinHeight, maxHeight)
}

func renderNotice(notice Notice, width int) string {
	if strings.TrimSpace(notice.Message) == "" {
		return ""
	}

	switch notice.Tone {
	case ToneSuccess:
		return theme.Success.Width(max(1, width)).Render(theme.Glyphs.Check + " " + notice.Message)
	case ToneWarning:
		return theme.Warning.Width(max(1, width)).Render("! " + notice.Message)
	case ToneDanger:
		return theme.Danger.Width(max(1, width)).Render(theme.Glyphs.Cross + " " + notice.Message)
	default:
		return theme.BodyStrong.Width(max(1, width)).Render(notice.Message)
	}
}

func renderWelcome(screen Screen, width int) string {
	title := strings.TrimSpace(screen.Title)
	if title == "" {
		title = "Welcome to Forged"
	}
	context := strings.TrimSpace(screen.Context)
	if context == "" {
		context = "Restore your synced vault or start fresh on this device"
	}

	if split := renderWelcomeSplit(title, context, screen.Options, width); split != "" {
		return split
	}

	contentWidth := max(1, width-stackedLeftInset-stackedRightInset)
	sections := []string{
		"",
		leftAlignBlock(width, theme.HeroTitle.Render(title), stackedLeftInset),
	}

	contextBlock := theme.Body.Width(min(contentWidth, 64)).Align(lipgloss.Left).Render(context)
	sections = append(sections, leftAlignBlock(width, contextBlock, stackedLeftInset))

	if len(screen.Options) > 0 {
		sections = append(sections, "", "", renderWelcomeCards(screen.Options, width, max(0, stackedLeftInset-1)))
	}

	return strings.Join(sections, "\n")
}

func renderWelcomeSplit(title string, context string, options []Option, width int) string {
	if len(options) != 2 {
		return ""
	}

	const (
		separatorWidth       = 3
		fixedCardWidth       = 36
		minLeftWidth         = 24
		rightPaneSidePadding = 2
	)

	cardBodyHeight := max(
		measureWelcomeCardBodyHeight(options[0], fixedCardWidth),
		measureWelcomeCardBodyHeight(options[1], fixedCardWidth),
	)
	topCard := renderWelcomeCard(options[0], fixedCardWidth, cardBodyHeight)
	bottomCard := renderWelcomeCard(options[1], fixedCardWidth, cardBodyHeight)

	rightStack := strings.Join([]string{topCard, bottomCard}, "\n")
	rightMinWidth := lipgloss.Width(rightStack) + rightPaneSidePadding*2
	available := width - separatorWidth
	if available < rightMinWidth+minLeftWidth {
		return ""
	}

	equalPaneWidth := available / 2
	leftWidth := equalPaneWidth
	rightWidth := available - leftWidth
	if equalPaneWidth < rightMinWidth {
		rightWidth = rightMinWidth
		leftWidth = available - rightWidth
		if leftWidth < minLeftWidth {
			return ""
		}
	}

	leftBlock := strings.Join([]string{
		theme.HeroTitle.Render(title),
		"",
		theme.Body.Width(max(18, min(leftWidth-4, 34))).Align(lipgloss.Center).Render(context),
	}, "\n")

	sectionHeight := max(16, lipgloss.Height(rightStack)+2, lipgloss.Height(leftBlock)+2)
	leftPane := lipgloss.Place(leftWidth, sectionHeight, lipgloss.Center, lipgloss.Center, leftBlock)
	rightPane := lipgloss.Place(rightWidth, sectionHeight, lipgloss.Center, lipgloss.Center, rightStack)
	separator := renderWelcomeSeparator(sectionHeight)

	row := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, separator, rightPane)
	return centerBlock(width, row)
}

func renderWelcomeCards(options []Option, width int, leftInset int) string {
	blocks := make([]string, 0, len(options)*2)
	availableWidth := max(1, width-leftInset-4)
	cardWidth := max(1, availableWidth-1)
	cardBodyHeight := 0
	for _, option := range options {
		cardBodyHeight = max(cardBodyHeight, measureWelcomeCardBodyHeight(option, cardWidth))
	}
	for index, option := range options {
		blocks = append(blocks, leftAlignBlock(width, renderWelcomeCard(option, cardWidth, cardBodyHeight), leftInset))
		if index < len(options)-1 {
			blocks = append(blocks, "")
		}
	}
	return strings.Join(blocks, "\n")
}

func renderWelcomeSeparator(height int) string {
	lines := make([]string, height)
	for index := range lines {
		lines[index] = " " + theme.HeaderSeparator.Render(theme.Glyphs.Vertical) + " "
	}
	return strings.Join(lines, "\n")
}

func renderWelcomeCard(option Option, cardWidth int, bodyHeight int) string {
	padding := []int{1, 2}
	borderColor := theme.ColorBorder
	titleStyle := theme.BodyStrong
	descriptionStyle := theme.Body

	if option.Selected {
		borderColor = theme.ColorAccent
		titleStyle = theme.Kicker
		descriptionStyle = theme.Body
	}

	if !option.Selected {
		descriptionStyle = theme.BodyMuted
	}

	frame := lipgloss.NewStyle().
		BorderStyle(theme.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(padding[0], padding[1]).
		Width(cardWidth)

	innerWidth := max(1, cardWidth-padding[1]*2)
	body := renderWelcomeCardBody(option, titleStyle, descriptionStyle, innerWidth)
	if bodyHeight > 0 {
		body = lipgloss.Place(innerWidth, bodyHeight, lipgloss.Left, lipgloss.Top, body)
	}

	return frame.Render(body)
}

func centerBlock(width int, block string) string {
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(block)
}

func leftAlignBlock(width int, block string, inset int) string {
	return lipgloss.NewStyle().
		Width(width).
		PaddingLeft(max(0, inset)).
		Align(lipgloss.Left).
		Render(block)
}

func measureWelcomeCardBodyHeight(option Option, cardWidth int) int {
	paddingRightLeft := 4
	innerWidth := max(1, cardWidth-paddingRightLeft)
	return lipgloss.Height(renderWelcomeCardBody(option, theme.BodyStrong, theme.Body, innerWidth))
}

func renderWelcomeCardBody(option Option, titleStyle lipgloss.Style, descriptionStyle lipgloss.Style, innerWidth int) string {
	body := []string{titleStyle.Width(innerWidth).Render(option.Label)}
	if strings.TrimSpace(option.Description) != "" {
		body = append(body, "")
		body = append(body, descriptionStyle.Width(innerWidth).Render(option.Description))
	}
	return strings.Join(body, "\n")
}
