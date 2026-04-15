package cmd

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/commandui"
)

type managerItem struct {
	Label string
	Run   func() error
}

type managerModel struct {
	title string
	items []managerItem

	width     int
	cursor    int
	selected  int
	cancelled bool
}

func newManagerModel(title string, items []managerItem) *managerModel {
	return &managerModel{
		title:    title,
		items:    items,
		selected: -1,
	}
}

func (m *managerModel) Init() tea.Cmd {
	return nil
}

func (m *managerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.items) > 0 {
				m.selected = m.cursor
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *managerModel) View() string {
	lines := []string{
		commandui.TitleStyle.Render(m.title),
		"",
		renderManagerItems(m.items, m.cursor),
		"",
		commandui.MutedStyle.Render("↑/↓ move  Enter select  Esc quit"),
	}

	return commandui.RenderContainer(m.width, strings.Join(lines, "\n"))
}

func renderManagerItems(items []managerItem, cursor int) string {
	var lines []string
	for i, item := range items {
		line := "  " + item.Label
		if i == cursor {
			line = commandui.SelectedItemStyle.Render("› " + item.Label)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func runManagerProgram(title string, items []managerItem) error {
	final, err := tea.NewProgram(newManagerModel(title, items)).Run()
	if err != nil {
		return err
	}

	model, ok := final.(*managerModel)
	if !ok {
		return nil
	}
	if model.cancelled || model.selected < 0 || model.selected >= len(items) {
		return nil
	}

	return items[model.selected].Run()
}
