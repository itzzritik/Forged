package gate

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/flame"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type Kind int

const (
	Launch Kind = iota
	Welcome
	RepairAccount
	Create
	Login
	Restore
	Unlock
	Busy
)

type ChooseMsg struct{ Login bool }
type SubmitMsg struct {
	Kind              Kind
	Password, Confirm []byte
}
type SystemAuthMsg struct{}
type UsePasswordMsg struct{}
type LoginKeyMsg struct{ Key string }
type QuitMsg struct{}
type BackMsg struct{}

type Model struct {
	kind       Kind
	cursor     int
	focus      int
	a, b       ui.Secret
	err        string
	errAt      int
	busy       string
	waiting    bool
	prompt     string
	trustKnown bool
	hasTrust   bool
	code, url  string
	status     string
	committing bool
	fire       *flame.Fire
	animate    bool
	noFire     bool
}

func New() *Model {
	m := &Model{animate: flame.Animate(), noFire: ui.ASCII() || os.Getenv("NO_COLOR") != ""}
	m.Show(Launch)
	return m
}

func (m *Model) Kind() Kind { return m.kind }

func (m *Model) Trusted() bool { return m.hasTrust }

func (m *Model) Show(k Kind) {
	m.Wipe()
	limit := 128
	if k == Create {
		limit = 129
	}
	*m = Model{kind: k, a: ui.NewSecret(limit), b: ui.NewSecret(limit), fire: m.fire, animate: m.animate, noFire: m.noFire}
	if k == Create {
		m.errAt = 1
	}
}

func (m *Model) SetBusy(text string) { m.busy = text }

func (m *Model) SetError(text string) {
	m.err = text
	m.errAt = 0
	if m.kind == Create {
		m.errAt = 1
	}
}

func (m *Model) SetLogin(code, url, status string, committing bool) {
	m.code, m.url, m.status, m.committing = code, url, status, committing
}

func (m *Model) SetUnlock(systemAuth bool, prompt string) {
	m.waiting, m.prompt = systemAuth, prompt
	if systemAuth {
		m.Wipe()
	}
}

func (m *Model) Wipe() {
	m.a.Wipe()
	m.b.Wipe()
}

func (m *Model) Tick() {
	if m.animate {
		m.fire.Step()
	}
}

func (m *Model) Quiet() bool {
	if m.err != "" || m.prompt != "" {
		return false
	}
	switch m.kind {
	case Launch, Busy:
		return true
	case Unlock:
		return m.waiting || m.busy != ""
	}
	return false
}

func (m *Model) Spinning() bool {
	return m.busy != "" || m.waiting || m.kind == Login || (m.kind == Launch || m.kind == Busy) && m.err == ""
}

func (m *Model) Refresh(st *core.State) {
	if m.kind == Unlock && !m.trustKnown {
		m.trustKnown = true
		m.hasTrust = st != nil && !st.Deps.Remote && st.Deps.HasLocalUnlockTrust != nil && st.Deps.HasLocalUnlockTrust()
	}
}

func (m *Model) exit(msg tea.Msg) tea.Cmd {
	m.Wipe()
	return core.Send(msg)
}

func (m *Model) Update(msg tea.Msg, st *core.State) tea.Cmd {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg:
	default:
		return nil
	}
	m.Refresh(st)
	if m.busy != "" && m.kind != Login {
		return nil
	}
	key := ""
	if k, ok := msg.(tea.KeyPressMsg); ok {
		key = k.String()
	}
	switch m.kind {
	case Welcome:
		switch key {
		case "up", "k", "down", "j":
			m.cursor = 1 - m.cursor
		case "enter":
			return core.Send(ChooseMsg{Login: m.cursor == 0})
		case "esc":
			return core.Send(QuitMsg{})
		}
	case RepairAccount:
		switch key {
		case "enter":
			return core.Send(ChooseMsg{Login: true})
		case "esc":
			return core.Send(QuitMsg{})
		}
	case Login:
		if m.committing {
			return nil
		}
		switch key {
		case "enter", "esc":
			return core.Send(LoginKeyMsg{Key: key})
		case "c", "C":
			return core.Send(LoginKeyMsg{Key: "c"})
		}
	case Create:
		return m.updateCreate(msg, key)
	case Unlock, Restore:
		return m.updatePassword(msg, key, st)
	}
	return nil
}

func (m *Model) edit(s *ui.Secret, msg tea.Msg) {
	if s.Update(msg) {
		m.err = ""
	}
}

func (m *Model) updateCreate(msg tea.Msg, key string) tea.Cmd {
	switch key {
	case "esc":
		return m.exit(BackMsg{})
	case "tab":
		m.focus = 1 - m.focus
		return nil
	case "enter":
		if m.focus == 0 {
			m.focus = 1
			return nil
		}
		return m.submitCreate()
	}
	if m.focus == 0 {
		m.edit(&m.a, msg)
	} else {
		m.edit(&m.b, msg)
	}
	return nil
}

func (m *Model) submitCreate() tea.Cmd {
	pw, confirm := m.a.Take(), m.b.Take()
	m.focus = 0
	msg, at := "", 0
	if err := core.ValidateNew(pw); err != nil {
		msg = err.Error()
	} else if !core.Same(pw, confirm) {
		msg, at = "Passwords do not match", 1
	}
	if msg != "" {
		clear(pw)
		clear(confirm)
		m.err, m.errAt = msg, at
		return nil
	}
	return core.Send(SubmitMsg{Kind: Create, Password: pw, Confirm: confirm})
}

func (m *Model) updatePassword(msg tea.Msg, key string, st *core.State) tea.Cmd {
	idle := !m.waiting && m.a.Len() == 0 && m.kind == Unlock && m.hasTrust
	switch key {
	case "esc":
		if m.waiting {
			return core.Send(UsePasswordMsg{})
		}
		if m.kind == Restore {
			return m.exit(BackMsg{})
		}
		return m.exit(QuitMsg{})
	case "tab":
		if idle {
			return core.Send(SystemAuthMsg{})
		}
		return nil
	case "enter":
		switch {
		case m.waiting || idle:
			return core.Send(SystemAuthMsg{})
		case m.a.Len() > 0:
			return core.Send(SubmitMsg{Kind: m.kind, Password: m.a.Take()})
		}
		return nil
	}
	if !m.waiting {
		m.edit(&m.a, msg)
	}
	return nil
}
