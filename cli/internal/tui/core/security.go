package core

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func LoadSecurityCmd(st *State) tea.Cmd {
	load, id := st.Deps.LoadSecurityState, NextID()
	return func() tea.Msg {
		state, err := load()
		if err != nil {
			err = fmt.Errorf("loading security settings: %w", err)
		}
		return SecurityMsg{ID: id, State: state, Err: err}
	}
}
