package keys

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
)

var prefetchSem = make(chan struct{}, 4)

type copyDoneMsg struct {
	label, action string
	since         core.ID
	err           error
}

func loadDetail(viewKey func(string) (actions.KeyDetail, error), name string) tea.Cmd {
	id := core.NextID()
	return func() tea.Msg {
		prefetchSem <- struct{}{}
		defer func() { <-prefetchSem }()
		d, err := viewKey(name)
		if err != nil {
			err = fmt.Errorf("reading key details: %w", err)
		}
		return core.DetailMsg{ID: id, Name: name, Detail: d, Err: err}
	}
}

func prefetch(st *core.State, inflight map[string]bool) tea.Cmd {
	var cmds []tea.Cmd
	for _, k := range st.Keys {
		if _, ok := st.Details[k.Name]; ok || inflight[k.Name] {
			continue
		}
		inflight[k.Name] = true
		cmds = append(cmds, loadDetail(st.Deps.ViewKey, k.Name))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func copyFingerprint(st *core.State, fingerprint string) tea.Cmd {
	copyText, since := st.Deps.CopyText, core.NextID()
	return func() tea.Msg {
		done := copyDoneMsg{label: "Fingerprint", action: "copy fingerprint", since: since}
		if err := copyText(fingerprint); err != nil {
			done.err = fmt.Errorf("copying fingerprint: %w", err)
		}
		return done
	}
}

func enableSigning(st *core.State, id core.ID, name string) tea.Cmd {
	enable := st.Deps.EnableCommitSigning
	return func() tea.Msg {
		status, err := enable(name)
		if err != nil {
			err = fmt.Errorf("enabling commit signing: %w", err)
		}
		return core.SigningMsg{ID: id, Status: status, Err: err}
	}
}
