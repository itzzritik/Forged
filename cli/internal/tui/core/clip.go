package core

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

var ErrTerminalClipboard = errors.New("no local clipboard")

// Copy falls back to the terminal's own clipboard (OSC 52) when this session has no local one.
func Copy(copyText func(string) error, text string, done func(error) tea.Msg) tea.Msg {
	if err := copyText(text); !errors.Is(err, ErrTerminalClipboard) {
		return done(err)
	}
	return tea.BatchMsg{tea.SetClipboard(text), Send(done(nil))}
}

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
		return Copy(copyText, public, func(err error) tea.Msg {
			if err != nil {
				err = fmt.Errorf("copying public key: %w", err)
			}
			return done(since, err)
		})
	}
}
