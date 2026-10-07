package components

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

type PasswordKind string

// Unlock fields keep truncating at this length so existing vaults still open.
const maxPasswordLength = 128

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
		input.EchoCharacter = []rune(theme.Glyphs.Mask)[0]
		input.Prompt = ""
		input.CharLimit = maxPasswordLength
		if kind == PasswordKindCreate || kind == PasswordKindChange && index > 0 {
			input.CharLimit = maxPasswordLength + 1
		}
		StyleTextInput(&input)
		input.SetValue("")
		input.SetWidth(32)
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
		p.fields[index].SetWidth(max(12, width))
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

func (p *PasswordInput) Clear() {
	p.clearValues()
	p.ClearStatus()
}

func (p *PasswordInput) FocusIndex() int {
	return p.focus
}

func (p *PasswordInput) FieldCount() int {
	return len(p.fields)
}

func (p *PasswordInput) IsEmpty() bool {
	for _, field := range p.fields {
		if field.Value() != "" {
			return false
		}
	}
	return true
}

func (p *PasswordInput) MoveNext() {
	p.moveFocus("down")
}

func (p *PasswordInput) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.SetWidth(max(12, msg.Width/2))
	case tea.KeyPressMsg:
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
		return nil, fmt.Errorf("Enter your master password")
	}

	if p.kind == PasswordKindCreate {
		if err := validateNewPassword(primary); err != nil {
			return nil, err
		}
		if primary != p.fields[1].Value() {
			return nil, fmt.Errorf("Passwords do not match")
		}
	}

	if p.kind == PasswordKindChange {
		if err := validateNewPassword(p.fields[1].Value()); err != nil {
			return nil, err
		}
		if p.fields[1].Value() != p.fields[2].Value() {
			return nil, fmt.Errorf("Passwords do not match")
		}
	}

	password := []byte(primary)
	p.clearValues()
	return password, nil
}

func (p *PasswordInput) SubmitChangePassword() ([]byte, []byte, error) {
	p.ClearStatus()
	if p.kind != PasswordKindChange {
		return nil, nil, fmt.Errorf("Change-password input is not active")
	}

	current := p.fields[0].Value()
	if len(current) == 0 {
		return nil, nil, fmt.Errorf("Enter your current master password")
	}

	next := p.fields[1].Value()
	if err := validateNewPassword(next); err != nil {
		return nil, nil, err
	}
	if next != p.fields[2].Value() {
		return nil, nil, fmt.Errorf("Passwords do not match")
	}

	currentPassword := []byte(current)
	newPassword := []byte(next)
	p.clearValues()
	return currentPassword, newPassword, nil
}

func (p *PasswordInput) clearValues() {
	for index := range p.fields {
		p.fields[index].Reset()
		p.fields[index].Blur()
	}
	p.focus = 0
	if len(p.fields) > 0 {
		p.fields[0].Focus()
	}
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
			theme.FieldValue.Render(theme.AdaptTextInputPlaceholder(field.View(), field.Value())),
			lineStyle.Render(strings.Repeat(theme.Glyphs.Horizontal, min(inputWidth, 36))),
		}
		if strings.TrimSpace(label) != "" {
			lines = append([]string{theme.FieldLabel.Width(inputWidth).Render(label)}, lines...)
		} else {
			lines = append([]string{""}, lines...)
		}

		sections = append(sections, strings.Join(lines, "\n"))
	}

	feedbackWidth := max(12, p.width)
	if p.err != "" {
		sections = append(sections, theme.Danger.Width(feedbackWidth).Render(theme.Glyphs.Cross+" "+sentenceCase(p.err)))
	} else if p.ok != "" {
		sections = append(sections, theme.Success.Width(feedbackWidth).Render(theme.Glyphs.Check+" "+sentenceCase(p.ok)))
	} else if p.info != "" {
		sections = append(sections, theme.BodyStrong.Width(feedbackWidth).Render(theme.Spinner.Render(spinner)+" "+sentenceCase(p.info)))
	} else {
		sections = append(sections, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func sentenceCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) == 0 || !unicode.IsLower(runes[0]) {
		return trimmed
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// A pasted trailing newline arrives as a space; reject it rather than store it.
func validateNewPassword(value string) error {
	switch length := utf8.RuneCountInString(value); {
	case length < 8:
		return fmt.Errorf("Use at least 8 characters")
	case length > maxPasswordLength:
		return fmt.Errorf("Use at most %d characters", maxPasswordLength)
	case strings.TrimSpace(value) != value:
		return fmt.Errorf("Remove spaces from the start and end")
	}
	return nil
}
