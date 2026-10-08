package core

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func ClipTick(id ID) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return ClipTickMsg{ID: id} })
}

func ClipClear(c *Clip) tea.Cmd {
	id, lease := c.ID, c.Lease
	return func() tea.Msg {
		if lease == nil {
			return ClipClearedMsg{ID: id}
		}
		cleared, err := lease.ClearIfUnchanged()
		return ClipClearedMsg{ID: id, Cleared: cleared, Err: err}
	}
}

func CopyPublic(st *State, name string, done func(since ID, err error) tea.Msg) tea.Cmd {
	copyText, viewKey, public, since := st.Deps.CopyText, st.Deps.ViewKey, st.Details[name].PublicKey, NextID()
	return func() tea.Msg {
		if public == "" {
			d, err := viewKey(name)
			if err != nil {
				return done(since, fmt.Errorf("reading public key: %w", err))
			}
			public = d.PublicKey
		}
		if err := copyText(public); err != nil {
			return done(since, fmt.Errorf("copying public key: %w", err))
		}
		return done(since, nil)
	}
}
