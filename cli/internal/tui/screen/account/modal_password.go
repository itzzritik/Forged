package account

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type passwordDoneMsg struct {
	id     core.ID
	detail string
	err    error
}

var passwordLabels = [3]string{"Current master password", "New master password", "Confirm new password"}

type passwordModal struct {
	owner  *Model
	fields [3]ui.Secret
	focus  int
	busy   bool
	id     core.ID
	errMsg string
}

func newPasswordModal(owner *Model) *passwordModal {
	return &passwordModal{owner: owner, fields: [3]ui.Secret{ui.NewSecret(128), ui.NewSecret(129), ui.NewSecret(129)}}
}

func (m *passwordModal) Spinning() bool { return m.busy }

func (m *passwordModal) wipe() {
	for i := range m.fields {
		m.fields[i].Wipe()
	}
}

func (m *passwordModal) Actions(*core.State) []core.Action {
	if m.busy {
		return nil
	}
	label := "Next"
	if m.focus == 2 {
		label = "Change"
	}
	return []core.Action{{Key: "enter", Label: label}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Cancel"}}
}

func (m *passwordModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case core.LockedMsg:
		m.wipe()
		m.focus, m.errMsg = 0, ""
	case passwordDoneMsg:
		if msg.id != m.id || !m.busy {
			return m, nil
		}
		m.busy = false
		if msg.err != nil {
			m.errMsg = st.Reporter.Report("account", "change master password", msg.err)
			m.focus = 0
			return m, nil
		}
		return m, core.Close(m)
	case tea.PasteMsg:
		if !m.busy {
			m.errMsg = ""
			m.fields[m.focus].Update(msg)
		}
	case tea.KeyPressMsg:
		if m.busy {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			m.wipe()
			return m, core.Close(m)
		case "tab", "down":
			m.focus = (m.focus + 1) % 3
		case "shift+tab", "up":
			m.focus = (m.focus + 2) % 3
		case "enter":
			if m.focus < 2 {
				m.focus++
				return m, nil
			}
			return m, m.submit(st)
		default:
			m.errMsg = ""
			m.fields[m.focus].Update(msg)
		}
	}
	return m, nil
}

func (m *passwordModal) validate() (focus int, err error) {
	if m.fields[0].Len() == 0 {
		return 0, errors.New("Enter your current master password")
	}
	next, confirm := m.fields[1].Peek(), m.fields[2].Peek()
	defer clear(next)
	defer clear(confirm)
	if err := core.ValidateNew(next); err != nil {
		return 1, err
	}
	if !core.Same(next, confirm) {
		return 2, errors.New("Passwords do not match")
	}
	return 0, nil
}

func (m *passwordModal) submit(st *core.State) tea.Cmd {
	if focus, err := m.validate(); err != nil {
		m.focus, m.errMsg = focus, err.Error()
		return nil
	}
	m.id, m.busy, m.errMsg = core.NextID(), true, ""
	m.owner.pwID = m.id
	st.Busy.PasswordChange = true
	cur, next := m.fields[0].Take(), m.fields[1].Take()
	m.fields[2].Wipe()
	change, id := st.Deps.ChangePassword, m.id
	return func() tea.Msg {
		defer clear(cur)
		defer clear(next)
		res, err := change(cur, next)
		if err != nil {
			return passwordDoneMsg{id: id, err: fmt.Errorf("changing master password: %w", err)}
		}
		return passwordDoneMsg{id: id, detail: res.Detail}
	}
}

func (m *passwordModal) status(inner, frame int) string {
	p := ui.P()
	switch {
	case m.busy:
		return ui.SpinnerGlyph(frame) + " " + ui.Paint("Changing master password", p.Text)
	case m.errMsg != "":
		return ui.Paint(ui.Trunc(m.errMsg, inner), p.Danger)
	}
	return ""
}

func (m *passwordModal) body(inner, limit, frame int) []string {
	p := ui.P()
	field := func(i int, label string) []string {
		return []string{ui.Paint(ui.Trunc(label, inner), p.Muted), m.fields[i].View(inner, i == m.focus && !m.busy)}
	}
	var lines []string
	switch {
	case limit >= 10:
		lines = append(lines, "")
		for i, l := range passwordLabels {
			lines = append(lines, field(i, l)...)
			lines = append(lines, "")
		}
		lines[len(lines)-1] = m.status(inner, frame)
	case limit >= 7:
		for i, l := range passwordLabels {
			lines = append(lines, field(i, l)...)
		}
		lines = append(lines, m.status(inner, frame))
	default:
		lines = append(field(m.focus, fmt.Sprintf("%s (%d/3)", passwordLabels[m.focus], m.focus+1)), m.status(inner, frame))
	}
	return lines[:min(len(lines), max(0, limit))]
}

func (m *passwordModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, m.body)
}

func (m *passwordModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Change master password", "", m.body)
}
