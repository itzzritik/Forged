package cmd

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/commandui"
)

type textPromptModel struct {
	title string
	body  string
	placeholder string

	value string
	width int

	cancelled bool
	submitted bool
	errText   string
}

func newTextPromptModel(title, body, placeholder, initialValue string) *textPromptModel {
	return &textPromptModel{
		title:       title,
		body:        body,
		placeholder: placeholder,
		value:       initialValue,
	}
}

func (m *textPromptModel) Init() tea.Cmd {
	return nil
}

func (m *textPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			value := strings.TrimSpace(m.value)
			if value == "" {
				m.errText = "A name is required."
				return m, nil
			}
			m.value = value
			m.submitted = true
			return m, tea.Quit
		case "backspace":
			m.value = trimTrailingRune(m.value)
			if strings.TrimSpace(m.value) != "" {
				m.errText = ""
			}
			return m, nil
		default:
			key := msg.String()
			if len([]rune(key)) == 1 {
				if len([]rune(m.value)) < 120 {
					m.value += key
				}
				m.errText = ""
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *textPromptModel) View() string {
	lines := []string{
		commandui.TitleStyle.Render(m.title),
	}
	if m.body != "" {
		lines = append(lines, "", commandui.MutedStyle.Render(m.body))
	}
	lines = append(lines, "", renderTextPromptValue(m.value, m.placeholder))
	if m.errText != "" {
		lines = append(lines, commandui.ErrorStyle.Render(m.errText))
	}
	lines = append(lines, "", commandui.MutedStyle.Render("Enter save  Esc cancel"))

	return commandui.RenderContainer(m.width, strings.Join(lines, "\n"))
}

func runTextPromptProgram(title, body, placeholder, initialValue string) (string, bool, error) {
	final, err := tea.NewProgram(newTextPromptModel(title, body, placeholder, initialValue)).Run()
	if err != nil {
		return "", false, err
	}

	model, ok := final.(*textPromptModel)
	if !ok || model.cancelled || !model.submitted {
		return "", false, nil
	}

	return model.value, true, nil
}

func runConfirmProgram(title, body, confirmLabel string) (bool, error) {
	confirmed := false
	items := []managerItem{
		{
			Label: confirmLabel,
			Run: func() error {
				confirmed = true
				return nil
			},
		},
		{
			Label: "Cancel",
			Run: func() error {
				return nil
			},
		},
	}

	if body == "" {
		return confirmed, runManagerProgram(title, items)
	}

	return confirmed, runFramedManagerProgram(title, body, items)
}

func renderTextPromptValue(value, placeholder string) string {
	cursor := commandui.AccentStyle.Render("█")
	if value == "" {
		if placeholder == "" {
			return "> " + cursor
		}
		return "> " + commandui.MutedStyle.Render(placeholder) + cursor
	}
	return "> " + value + cursor
}

func trimTrailingRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}
