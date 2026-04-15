package launcher

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/commandui"
	"github.com/itzzritik/forged/cli/internal/readiness"
)

type startupFunc func() (readiness.Snapshot, readiness.RepairSummary, error)

type startupMsg struct {
	snapshot readiness.Snapshot
	summary  readiness.RepairSummary
	err      error
}

type Model struct {
	startup startupFunc
	spinner spinner.Model
	flash   string

	width    int
	phase    string
	snapshot readiness.Snapshot
	summary  readiness.RepairSummary
	menu     []MenuItem
	cursor   int

	selected  ActionID
	cancelled bool
	err       error
}

func NewModel(startup startupFunc, flash string) *Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = accentStyle

	return &Model{
		startup: startup,
		spinner: spin,
		flash:   flash,
		phase:   "loader",
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runStartup())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case spinner.TickMsg:
		if m.phase != "loader" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case startupMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, tea.Quit
		}
		m.snapshot = msg.snapshot
		m.summary = msg.summary
		m.menu = BuildMenu(msg.snapshot)
		m.phase = "menu"
		m.cursor = 0
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.phase == "menu" && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.phase == "menu" && m.cursor < len(m.menu)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if m.phase == "menu" && len(m.menu) > 0 {
				m.selected = m.menu[m.cursor].ID
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *Model) View() string {
	if m.phase == "loader" {
		return m.renderContainer(strings.Join([]string{
			titleStyle.Render("Preparing Forged"),
			"",
			m.renderLoaderRow("Vault"),
			m.renderLoaderRow("Service"),
			m.renderLoaderRow("SSH"),
			m.renderLoaderRow("Agent"),
			m.renderLoaderRow("Account"),
		}, "\n"))
	}

	lines := []string{titleStyle.Render("Forged")}
	if m.flash != "" {
		lines = append(lines, m.flash, "")
	}
	if len(m.summary.Fixed) > 0 {
		lines = append(lines, successStyle.Render("Ready after repair"), mutedStyle.Render(strings.Join(m.summary.Fixed, ", ")), "")
	}
	lines = append(lines, mutedStyle.Render(m.stateSubtitle()), "")
	lines = append(lines, m.renderMenu())
	lines = append(lines, "",
		commandui.RenderFooter(
			commandui.FooterAction("↑/↓", "Move"),
			commandui.FooterAction("Enter", "Select"),
			commandui.FooterAction("Esc", "Exit"),
		),
	)

	return m.renderContainer(strings.Join(lines, "\n"))
}

func (m *Model) renderLoaderRow(label string) string {
	return fmt.Sprintf("%s  %s", mutedStyle.Render(label), m.spinner.View())
}

func (m *Model) renderMenu() string {
	var lines []string
	for i, item := range m.menu {
		line := "  " + item.Label
		if i == m.cursor {
			line = selectedItemStyle.Render("› " + item.Label)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderContainer(content string) string {
	return commandui.RenderContainer(m.width, content)
}

func (m *Model) stateSubtitle() string {
	switch m.snapshot.State {
	case readiness.StateUninitialized:
		return "Choose how to start"
	case readiness.StateReadyEmpty:
		return "Your vault is ready. Choose what to do next"
	case readiness.StateReady:
		return "Everything looks healthy. Choose what to do next"
	case readiness.StateDegraded:
		return "Forged needs attention. Choose what to do next"
	case readiness.StateBlocked:
		return warnStyle.Render("Forged needs attention before it is fully usable")
	default:
		return "Choose what to do next"
	}
}

func (m *Model) runStartup() tea.Cmd {
	return func() tea.Msg {
		snapshot, summary, err := m.startup()
		return startupMsg{snapshot: snapshot, summary: summary, err: err}
	}
}
