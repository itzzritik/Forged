package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type PasswordKind string

const (
	PasswordKindUnlock PasswordKind = "unlock"
	PasswordKindCreate PasswordKind = "create"
)

type PasswordInput struct {
	kind  PasswordKind
	width int
	focus int
	err   string
	ok    string

	fields []textinput.Model
}

func NewUnlockPasswordInput() *PasswordInput {
	return newPasswordInput(PasswordKindUnlock)
}

func NewCreatePasswordInput() *PasswordInput {
	return newPasswordInput(PasswordKindCreate)
}

func newPasswordInput(kind PasswordKind) *PasswordInput {
	fieldCount := 1
	if kind == PasswordKindCreate {
		fieldCount = 2
	}

	fields := make([]textinput.Model, 0, fieldCount)
	for index := 0; index < fieldCount; index++ {
		input := textinput.New()
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
		input.Prompt = ""
		input.CharLimit = 128
		input.Cursor.Style = theme.FooterKey
		input.TextStyle = theme.FieldValue
		input.PlaceholderStyle = theme.BodyMuted
		input.SetValue("")
		input.Width = 32
		fields = append(fields, input)
	}
	fields[0].Placeholder = "Enter master password"
	if kind == PasswordKindCreate {
		fields[1].Placeholder = "Confirm master password"
	}
	fields[0].Focus()

	return &PasswordInput{
		kind:   kind,
		width:  32,
		fields: fields,
	}
}

func (p *PasswordInput) Init() tea.Cmd {
	return textinput.Blink
}

func (p *PasswordInput) SetWidth(width int) {
	if width <= 0 {
		return
	}
	p.width = width
	for index := range p.fields {
		p.fields[index].Width = max(12, width)
	}
}

func (p *PasswordInput) SetError(message string) {
	p.err = message
	if message != "" {
		p.ok = ""
	}
}

func (p *PasswordInput) SetSuccess(message string) {
	p.ok = message
	if message != "" {
		p.err = ""
	}
}

func (p *PasswordInput) ClearStatus() {
	p.err = ""
	p.ok = ""
}

func (p *PasswordInput) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.SetWidth(max(20, msg.Width/2))
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "up", "down":
			p.moveFocus(msg.String())
			return nil
		}
	}

	cmds := make([]tea.Cmd, 0, len(p.fields))
	for index := range p.fields {
		var cmd tea.Cmd
		p.fields[index], cmd = p.fields[index].Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (p *PasswordInput) moveFocus(key string) {
	if len(p.fields) <= 1 {
		return
	}

	next := p.focus
	if key == "shift+tab" || key == "up" {
		next--
	} else {
		next++
	}
	if next < 0 {
		next = len(p.fields) - 1
	}
	if next >= len(p.fields) {
		next = 0
	}
	p.focus = next

	for index := range p.fields {
		if index == p.focus {
			p.fields[index].Focus()
			continue
		}
		p.fields[index].Blur()
	}
}

func (p *PasswordInput) Submit() ([]byte, error) {
	p.ClearStatus()

	primary := p.fields[0].Value()
	if len(primary) == 0 {
		return nil, fmt.Errorf("enter your master password")
	}

	if p.kind == PasswordKindCreate {
		if len(primary) < 8 {
			return nil, fmt.Errorf("use at least 8 characters")
		}
		if primary != p.fields[1].Value() {
			return nil, fmt.Errorf("passwords do not match")
		}
	}

	return []byte(primary), nil
}

func (p *PasswordInput) View(labels ...string) string {
	sections := make([]string, 0, len(p.fields)+1)
	for index, field := range p.fields {
		label := "Master password"
		if p.kind == PasswordKindCreate && index == 1 {
			label = "Confirm password"
		}
		if index < len(labels) && strings.TrimSpace(labels[index]) != "" {
			label = labels[index]
		}

		lineStyle := theme.FieldLineIdle
		if index == p.focus {
			lineStyle = theme.FieldLineActive
		}

		inputWidth := max(12, p.width)
		sections = append(sections, strings.Join([]string{
			theme.FieldLabel.Render(label),
			theme.FieldValue.Render(field.View()),
			lineStyle.Render(strings.Repeat("─", min(inputWidth, 36))),
		}, "\n"))
	}

	if p.err != "" {
		sections = append(sections, theme.Danger.Render("✕ "+p.err))
	} else if p.ok != "" {
		sections = append(sections, theme.Success.Render("✓ "+p.ok))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
