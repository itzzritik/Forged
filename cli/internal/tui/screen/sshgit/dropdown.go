package sshgit

import (
	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
)

type signChoiceMsg struct {
	name string
	off  bool
}

func (m *Model) openDropdown(st *core.State) tea.Cmd {
	if m.signBusy || st.Busy.SSHToggle {
		return nil
	}
	raw := make([]string, len(st.Keys))
	for i, k := range st.Keys {
		raw[i] = k.Name
	}
	cur := -1
	switch {
	case !st.SigningLoaded:
	case st.Signing.Mode == actions.CommitSigningForged:
		for i, n := range raw {
			if n == st.Signing.KeyName {
				cur = i
			}
		}
	case st.Signing.Mode == actions.CommitSigningOff:
		cur = len(raw)
	}
	d := &widget.Dropdown{
		Items: raw, Tail: []string{"Off"}, Cur: cur, Anchor: m.signingY(st), MaxW: 31,
		Choose: func(i int) tea.Msg {
			if i < len(raw) {
				return signChoiceMsg{name: raw[i]}
			}
			return signChoiceMsg{off: true}
		},
	}
	return d.Open(st)
}
