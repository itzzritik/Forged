package keys

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type nameModal struct {
	old    string
	form   *huh.Form
	value  string
	width  int
	id     core.ID
	busy   bool
	errMsg string
}

type nameDoneMsg struct {
	id        core.ID
	old, name string
	err       error
}

func NewKeyModal() widget.Modal { return newNameModal("") }

func RenameModal(name string) widget.Modal { return newNameModal(name) }

func newNameModal(old string) *nameModal {
	m := &nameModal{old: old, value: ui.Sanitize(old)}
	in := huh.NewInput().
		Value(&m.value).
		CharLimit(128).
		Prompt(ui.G.Next + " ").
		Validate(func(s string) error {
			s = strings.TrimSpace(ui.Sanitize(s))
			switch {
			case s == "":
				return errors.New("Enter a key name")
			case old != "" && s == strings.TrimSpace(ui.Sanitize(old)):
				return errors.New("Enter a different key name")
			}
			return nil
		})
	m.form = huh.NewForm(huh.NewGroup(in)).
		WithTheme(ui.HuhTheme()).
		WithKeyMap(ui.HuhKeyMap()).
		WithShowHelp(false).
		WithShowErrors(false)
	m.form.Init()
	return m
}

func (m *nameModal) rename() bool { return m.old != "" }

func (m *nameModal) Actions(*core.State) []core.Action {
	if m.busy {
		return nil
	}
	label := "Create"
	if m.rename() {
		label = "Rename"
	}
	return []core.Action{{Key: "enter", Label: label}, {Key: "esc", Label: "Cancel"}}
}

func (m *nameModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case nameDoneMsg:
		if msg.id != m.id || !m.busy {
			return m, nil
		}
		m.busy = false
		if msg.err != nil {
			action := "generate key"
			if m.rename() {
				action = "rename key"
			}
			m.errMsg = st.Reporter.Report("keys", action, msg.err)
			return m, nil
		}
		text := "Key created"
		if m.rename() {
			text = "Key renamed"
		}
		return m, tea.Sequence(
			core.Close(m),
			core.Send(core.SwitchTabMsg{Tab: core.TabKeys, Select: msg.name}),
			core.Toast(text, ui.ToneGood),
		)
	case tea.KeyPressMsg:
		if m.busy {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			return m, core.Close(m)
		case "tab", "shift+tab", "up", "down":
			return m, nil
		case "enter":
			m.form.Update(msg)
			if len(m.form.Errors()) > 0 {
				return m, nil
			}
			return m, m.submit(st)
		}
		m.errMsg = ""
	case tea.PasteMsg:
		if m.busy {
			return m, nil
		}
		msg.Content = ui.Sanitize(msg.Content)
		m.errMsg = ""
		_, cmd := m.form.Update(msg)
		return m, cmd
	default:
		if m.busy {
			return m, nil
		}
	}
	_, cmd := m.form.Update(msg)
	return m, cmd
}

func (m *nameModal) submit(st *core.State) tea.Cmd {
	name := strings.TrimSpace(ui.Sanitize(m.value))
	m.id, m.busy, m.errMsg = core.NextID(), true, ""
	id, old := m.id, m.old
	generate, rename := st.Deps.GenerateKey, st.Deps.RenameKey
	return func() tea.Msg {
		if old == "" {
			res, err := generate(name)
			if err != nil {
				return nameDoneMsg{id: id, err: fmt.Errorf("generating key: %w", err)}
			}
			return nameDoneMsg{id: id, name: res.Name}
		}
		res, err := rename(old, name)
		if err != nil {
			return nameDoneMsg{id: id, err: fmt.Errorf("renaming key: %w", err)}
		}
		return nameDoneMsg{id: id, old: old, name: res.NewName}
	}
}

func (m *nameModal) Spinning() bool { return m.busy }

func (m *nameModal) Capturing() bool { return true }

func (m *nameModal) body(inner, limit, frame int) []string {
	p := ui.P()
	if m.width != inner {
		m.width = inner
		m.form.WithWidth(inner)
	}
	sub := "Create a new SSH key"
	busy := "Generating key"
	if m.rename() {
		sub = "Choose a new name for " + ui.Sanitize(m.old)
		busy = "Saving key name"
	}
	if !m.busy {
		busy = ""
	}
	errText := m.errMsg
	if errText == "" && !m.busy {
		if errs := m.form.Errors(); len(errs) > 0 {
			errText = errs[0].Error()
		}
	}
	lines := []string{ui.Paint(ui.Trunc(sub, inner), p.Muted), "", strings.Split(m.form.View(), "\n")[0]}
	lines = append(lines, status(inner, frame, busy, errText)...)
	return pad(lines, limit)
}

func (m *nameModal) View(st *core.State, w, h int) string {
	title := "New key"
	if m.rename() {
		title = "Rename key"
	}
	return widget.ModalView(st, w, h, title, "", m.body)
}

func (m *nameModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, m.body)
}
