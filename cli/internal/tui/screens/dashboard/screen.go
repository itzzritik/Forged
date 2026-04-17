package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type Tone string

const (
	ToneNone    Tone = "none"
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

type Notice struct {
	Message string
	Tone    Tone
}

type Issue struct {
	Title  string
	Detail string
}

type Option struct {
	Label    string
	Description string
	Primary  bool
	Selected bool
}

type Screen struct {
	Title       string
	Context     string
	Snapshot    readiness.Snapshot
	Options     []Option
	Issues      []Issue
	ShowSummary bool
	Notice      Notice
}

func Render(screen Screen, width int) string {
	if len(screen.Options) > 0 {
		return renderWelcome(screen, width)
	}

	sections := make([]string, 0, 6)
	if notice := renderNotice(screen.Notice); notice != "" {
		sections = append(sections, notice)
	}

	if strings.TrimSpace(screen.Context) != "" {
		sections = append(sections, theme.Body.Width(max(28, min(width, theme.HeroMaxWidth))).Render(screen.Context))
	}

	if len(screen.Options) > 0 {
		sections = append(sections, "", renderOptions(screen.Options))
	}

	if len(screen.Issues) > 0 {
		sections = append(sections, "", renderIssues(screen.Issues))
	}

	if screen.ShowSummary {
		sections = append(sections, "", renderSummary(screen.Snapshot))
	}
	return strings.Join(sections, "\n")
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

func renderOptions(options []Option) string {
	lines := []string{theme.SectionTitle.Render("Choose how to start")}
	for _, option := range options {
		line := "  " + theme.Body.Render(option.Label)
		if option.Selected {
			line = theme.Kicker.Render("›") + " " + theme.BodyStrong.Render(option.Label)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
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

	const (
		stackedLeftInset  = 2
		stackedRightInset = 4
	)
	contentWidth := max(20, width-stackedLeftInset-stackedRightInset)
	sections := []string{
		"",
		leftAlignBlock(width, theme.HeroTitle.Render(title), stackedLeftInset),
	}

	contextBlock := theme.Body.Width(min(contentWidth, 64)).Align(lipgloss.Left).Render(context)
	sections = append(sections, leftAlignBlock(width, contextBlock, stackedLeftInset))

	if len(screen.Options) > 0 {
		sections = append(sections, "")
		sections = append(sections, "")
		sections = append(sections, renderWelcomeCards(screen.Options, width, max(0, stackedLeftInset-1)))
	}

	return strings.Join(sections, "\n")
}

func renderWelcomeSplit(title string, context string, options []Option, width int) string {
	if len(options) != 2 {
		return ""
	}

	const (
		separatorWidth      = 3
		fixedCardWidth      = 36
		minLeftWidth        = 24
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

func renderSummary(snapshot readiness.Snapshot) string {
	daemonState := "Offline"
	if snapshot.Service.Running {
		daemonState = "Running"
	}

	socketState := "Waiting"
	if snapshot.IPCSocketReady && snapshot.AgentSocketReady {
		socketState = "Ready"
	}

	sshState := "Not active"
	if snapshot.SSHEnabled && snapshot.ManagedConfigReady {
		sshState = "Active"
	}

	syncState := "Not linked"
	if snapshot.LoggedIn {
		syncState = "Linked"
	}

	rows := []struct {
		label string
		value string
	}{
		{label: "State", value: string(snapshot.State)},
		{label: "Keys", value: fmt.Sprintf("%d loaded", snapshot.KeyCount)},
		{label: "Daemon", value: daemonState},
		{label: "Sockets", value: socketState},
		{label: "SSH", value: sshState},
		{label: "Sync", value: syncState},
	}

	lines := []string{theme.SectionTitle.Render("Machine")}
	for _, row := range rows {
		left := theme.RowLabel.Render(strings.ToUpper(row.label))
		right := theme.RowValue.Render(row.value)
		gap := max(2, 14-lipgloss.Width(left))
		lines = append(lines, left+strings.Repeat(" ", gap)+right)
	}
	return strings.Join(lines, "\n")
}

func renderIssues(issues []Issue) string {
	lines := []string{theme.SectionTitle.Render("Issues")}
	for index, issue := range issues {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, theme.Danger.Render("✕")+" "+theme.BodyStrong.Render(issue.Title))
		if strings.TrimSpace(issue.Detail) != "" {
			lines = append(lines, "  "+theme.Body.Render(issue.Detail))
		}
	}
	return strings.Join(lines, "\n")
}
