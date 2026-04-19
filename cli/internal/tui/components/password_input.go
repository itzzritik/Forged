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
	PasswordKindChange PasswordKind = "change"
)

type PasswordInput struct {
	kind  PasswordKind
	width int
	focus int
	err   string
	ok    string
	info  string

	fields []textinput.Model
}

func NewUnlockPasswordInput() *PasswordInput {
	return newPasswordInput(PasswordKindUnlock)
}

func NewCreatePasswordInput() *PasswordInput {
	return newPasswordInput(PasswordKindCreate)
}

func NewChangePasswordInput() *PasswordInput {
	return newPasswordInput(PasswordKindChange)
}

func newPasswordInput(kind PasswordKind) *PasswordInput {
	fieldCount := 1
	if kind == PasswordKindCreate {
		fieldCount = 2
	}
	if kind == PasswordKindChange {
		fieldCount = 3
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
	if kind == PasswordKindChange {
		fields[0].Placeholder = "Current master password"
		fields[1].Placeholder = "New master password"
		fields[2].Placeholder = "Confirm new password"
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
		p.info = ""
	}
}

func (p *PasswordInput) SetSuccess(message string) {
	p.ok = message
	if message != "" {
		p.err = ""
		p.info = ""
	}
}

func (p *PasswordInput) SetInfo(message string) {
	p.info = message
	if message != "" {
		p.err = ""
		p.ok = ""
	}
}

func (p *PasswordInput) ClearStatus() {
	p.err = ""
	p.ok = ""
	p.info = ""
}

func (p *PasswordInput) FocusIndex() int {
	return p.focus
}

func (p *PasswordInput) FieldCount() int {
	return len(p.fields)
}

func (p *PasswordInput) MoveNext() {
	p.moveFocus("down")
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

	if p.kind == PasswordKindChange {
		if len(p.fields[1].Value()) < 8 {
			return nil, fmt.Errorf("use at least 8 characters")
		}
		if p.fields[1].Value() != p.fields[2].Value() {
			return nil, fmt.Errorf("passwords do not match")
		}
	}

	return []byte(primary), nil
}

func (p *PasswordInput) SubmitChangePassword() ([]byte, []byte, error) {
	p.ClearStatus()
	if p.kind != PasswordKindChange {
		return nil, nil, fmt.Errorf("change-password input is not active")
	}

	current := p.fields[0].Value()
	if len(current) == 0 {
		return nil, nil, fmt.Errorf("enter your current master password")
	}

	next := p.fields[1].Value()
	if len(next) < 8 {
		return nil, nil, fmt.Errorf("use at least 8 characters")
	}
	if next != p.fields[2].Value() {
		return nil, nil, fmt.Errorf("passwords do not match")
	}

	return []byte(current), []byte(next), nil
}

func (p *PasswordInput) View(spinner string, labels ...string) string {
	sections := make([]string, 0, len(p.fields)+1)
	for index, field := range p.fields {
		label := "Master password"
		if p.kind == PasswordKindCreate && index == 1 {
			label = "Confirm password"
		}
		if p.kind == PasswordKindChange {
			switch index {
			case 0:
				label = "Current password"
			case 1:
				label = "New password"
			case 2:
				label = "Confirm password"
			}
		}
		if index < len(labels) {
			label = labels[index]
		}

		lineStyle := theme.FieldLineIdle
		if index == p.focus {
			lineStyle = theme.FieldLineActive
		}

		inputWidth := max(12, p.width)
		lines := []string{
			theme.FieldValue.Render(field.View()),
			lineStyle.Render(strings.Repeat("─", min(inputWidth, 36))),
		}
		if strings.TrimSpace(label) != "" {
			lines = append([]string{theme.FieldLabel.Render(label)}, lines...)
		} else {
			lines = append([]string{""}, lines...)
		}

		sections = append(sections, strings.Join(lines, "\n"))
	}

	if p.err != "" {
		sections = append(sections, theme.Danger.Render("✕ "+p.err))
	} else if p.ok != "" {
		sections = append(sections, theme.Success.Render("✓ "+p.ok))
	} else if p.info != "" {
		sections = append(sections, theme.BodyStrong.Render(theme.Spinner.Render(spinner)+" "+p.info))
	} else {
		sections = append(sections, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
