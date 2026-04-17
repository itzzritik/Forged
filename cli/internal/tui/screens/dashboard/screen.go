package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Tone string

const (
	ToneNone    Tone = "none"
	ToneAccent  Tone = "accent"
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

type Area struct {
	Label       string
	Summary     string
	Description string
	Selected    bool
}

type Screen struct {
	Title   string
	Context string
	Notice  Notice
	Options []Option
	Areas   []Area
}

const (
	stackedLeftInset   = 2
	stackedRightInset  = 4
	dashboardRightPad  = 2
	gridColumnGap      = 3
	gridMinWidth       = 74
	gridCardMinWidth   = 24
	gridCardBodyHeight = 3
)

func Render(screen Screen, width int) string {
	if len(screen.Options) > 0 {
		return renderWelcome(screen, width)
	}
	if len(screen.Areas) > 0 {
		return renderDashboard(screen, width)
	}

	sections := make([]string, 0, 2)
	if notice := renderNotice(screen.Notice); notice != "" {
		sections = append(sections, notice)
	}
	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}
	return strings.Join(sections, "\n")
}

func renderDashboard(screen Screen, width int) string {
	safeWidth := max(20, width-dashboardRightPad)
	sections := make([]string, 0, 3)
	if notice := renderNotice(screen.Notice); notice != "" {
		sections = append(sections, notice, "")
	}

	sections = append(sections, renderAreas(screen.Areas, safeWidth))

	return strings.Join(sections, "\n")
}

func renderSelectedAreaDescription(areas []Area, width int) string {
	area := selectedArea(areas)
	if area == nil || strings.TrimSpace(area.Description) == "" {
		return ""
	}
	lineWidth := min(width, theme.HeroMaxWidth+6)
	return theme.BodyMuted.Width(max(24, lineWidth)).Render(area.Description)
}

func renderAreas(areas []Area, width int) string {
	if len(areas) == 0 {
		return ""
	}

	columns := AreaColumns(width, len(areas))
	if columns == 1 {
		return renderAreaStack(areas, width)
	}
	return renderAreaGrid(areas, width, columns)
}

func AreaColumns(width int, count int) int {
	if count < 2 || width < gridMinWidth {
		return 1
	}
	return 2
}

func renderAreaStack(areas []Area, width int) string {
	cardWidth := max(gridCardMinWidth, width)
	blocks := make([]string, 0, len(areas)*2)
	for index, area := range areas {
		blocks = append(blocks, renderAreaCard(area, cardWidth))
		if index < len(areas)-1 {
			blocks = append(blocks, "")
		}
	}
	return strings.Join(blocks, "\n")
}

func renderAreaGrid(areas []Area, width int, columns int) string {
	cardWidth := max(gridCardMinWidth, (width-gridColumnGap)/columns)
	rows := make([]string, 0, (len(areas)+columns-1)/columns)
	for start := 0; start < len(areas); start += columns {
		rowCards := make([]string, 0, columns)
		for offset := 0; offset < columns; offset++ {
			index := start + offset
			if index >= len(areas) {
				rowCards = append(rowCards, strings.Repeat(" ", cardWidth))
				continue
			}
			rowCards = append(rowCards, renderAreaCard(areas[index], cardWidth))
		}
		parts := make([]string, 0, len(rowCards)*2)
		for index, card := range rowCards {
			if index > 0 {
				parts = append(parts, strings.Repeat(" ", gridColumnGap))
			}
			parts = append(parts, card)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	}
	return strings.Join(rows, "\n\n")
}

func renderAreaCard(area Area, cardWidth int) string {
	borderColor := lipgloss.Color(theme.ColorBorder)
	titleStyle := theme.BodyStrong

	if area.Selected {
		borderColor = lipgloss.Color(theme.ColorAccent)
		titleStyle = theme.Kicker
	}

	frame := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(cardWidth)

	innerWidth := max(16, cardWidth-4)
	lines := []string{titleStyle.Render(area.Label)}
	if strings.TrimSpace(area.Summary) != "" {
		lines = append(lines, "")
	}
	if strings.TrimSpace(area.Summary) != "" {
		lines = append(lines, theme.BodyMuted.Width(innerWidth).Render(area.Summary))
	}
	body := strings.Join(lines, "\n")
	body = lipgloss.Place(innerWidth, gridCardBodyHeight, lipgloss.Left, lipgloss.Top, body)
	return frame.Render(body)
}

func selectedArea(areas []Area) *Area {
	for index := range areas {
		if areas[index].Selected {
			return &areas[index]
		}
	}
	if len(areas) == 0 {
		return nil
	}
	return &areas[0]
}

func renderNotice(notice Notice) string {
	if strings.TrimSpace(notice.Message) == "" {
		return ""
	}

	switch notice.Tone {
	case ToneSuccess:
		return theme.Success.Render("✓ " + notice.Message)
	case ToneWarning:
		return theme.Warning.Render("! " + notice.Message)
	case ToneDanger:
		return theme.Danger.Render("✕ " + notice.Message)
	default:
		return theme.BodyStrong.Render(notice.Message)
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

	contentWidth := max(20, width-stackedLeftInset-stackedRightInset)
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
	availableWidth := max(24, width-leftInset-4)
	cardWidth := max(24, availableWidth-1)
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
		lines[index] = " " + theme.HeaderSeparator.Render("│") + " "
	}
	return strings.Join(lines, "\n")
}

func renderWelcomeCard(option Option, cardWidth int, bodyHeight int) string {
	padding := []int{1, 2}
	borderColor := lipgloss.Color(theme.ColorBorder)
	titleStyle := theme.BodyStrong
	descriptionStyle := theme.Body

	if option.Selected {
		borderColor = lipgloss.Color(theme.ColorAccent)
		titleStyle = theme.Kicker
		descriptionStyle = theme.Body
	}

	if !option.Selected {
		descriptionStyle = theme.BodyMuted
	}

	frame := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(padding[0], padding[1]).
		Width(cardWidth)

	innerWidth := max(16, cardWidth-padding[1]*2)
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
	innerWidth := max(16, cardWidth-paddingRightLeft)
	return lipgloss.Height(renderWelcomeCardBody(option, theme.BodyStrong, theme.Body, innerWidth))
}

func renderWelcomeCardBody(option Option, titleStyle lipgloss.Style, descriptionStyle lipgloss.Style, innerWidth int) string {
	body := []string{titleStyle.Render(option.Label)}
	if strings.TrimSpace(option.Description) != "" {
		body = append(body, "")
		body = append(body, descriptionStyle.Width(innerWidth).Render(option.Description))
	}
	return strings.Join(body, "\n")
}
