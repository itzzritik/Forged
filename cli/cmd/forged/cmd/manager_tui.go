package cmd

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/commandui"
)

var errManagerContinue = errors.New("continue manager")

type managerItem struct {
	Label string
	Run   func() error
}

type managerModel struct {
	title string
	body  string
	items []managerItem

	width     int
	cursor    int
	selected  int
	cancelled bool
}

func newManagerModel(title, body string, items []managerItem) *managerModel {
	return &managerModel{
		title:    title,
		body:     body,
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
	lines := []string{commandui.TitleStyle.Render(m.title)}
	if m.body != "" {
		lines = append(lines, "", commandui.MutedStyle.Render(m.body))
	}
	lines = append(lines,
		"",
		renderManagerItems(m.items, m.cursor),
		"",
		commandui.RenderFooter(
			commandui.FooterAction("↑/↓", "Move"),
			commandui.FooterAction("Enter", "Select"),
			commandui.FooterAction("Esc", "Exit"),
		),
	)

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
	return runFramedManagerProgram(title, "", items)
}

func runManagerSelectionProgram(title string, items []managerItem) (int, bool, error) {
	return runFramedManagerSelectionProgram(title, "", items)
}

func runFramedManagerProgram(title, body string, items []managerItem) error {
	selected, cancelled, err := runFramedManagerSelectionProgram(title, body, items)
	if err != nil {
		return err
	}
	if cancelled || selected < 0 || selected >= len(items) {
		return nil
	}

	return items[selected].Run()
}

func runFramedManagerSelectionProgram(title, body string, items []managerItem) (int, bool, error) {
	final, err := tea.NewProgram(newManagerModel(title, body, items)).Run()
	if err != nil {
		return -1, false, err
	}

	model, ok := final.(*managerModel)
	if !ok {
		return -1, false, nil
	}
	return model.selected, model.cancelled, nil
}
