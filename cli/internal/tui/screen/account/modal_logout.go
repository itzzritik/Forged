package account

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type logoutDoneMsg struct {
	id  core.ID
	err error
}

type logoutModal struct {
	owner  *Model
	cancel bool
	busy   bool
	id     core.ID
}

func (l *logoutModal) Spinning() bool { return l.busy }

func (l *logoutModal) Actions(*core.State) []core.Action {
	if l.busy {
		return nil
	}
	if l.cancel {
		return []core.Action{{Key: "enter", Label: "Cancel"}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Cancel"}}
	}
	return []core.Action{{Key: "enter", Label: "Log out", Danger: true}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Cancel"}}
}

func logoutWarning(err error) string {
	syncPending := errors.Is(err, actions.ErrAccountClearSyncCleanupPending)
	secretPending := errors.Is(err, actions.ErrAccountClearCredentialSecretCleanupPending)
	switch {
	case syncPending && secretPending:
		return "Signed out. Local sync cleanup is incomplete and a retired credential could not be removed. Resolve both before switching accounts."
	case syncPending:
		return "Signed out. Local sync cleanup is incomplete. Review it before switching accounts."
	case secretPending:
		return "Signed out. A retired local credential artifact could not be removed. Resolve it before switching accounts."
	case errors.Is(err, actions.ErrAccountClearCommittedUnconfirmed):
		return "Signed out locally, but Forged could not confirm local cleanup. Restart Forged before switching accounts."
	}
	return ""
}

func (l *logoutModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case logoutDoneMsg:
		if msg.id != l.id || !l.busy {
			return l, nil
		}
		l.busy = false
		return l, core.Close(l)
	case tea.KeyPressMsg:
		if l.busy {
			return l, nil
		}
		switch msg.String() {
		case "esc":
			return l, core.Close(l)
		case "tab", "shift+tab", "left", "right":
			l.cancel = !l.cancel
		case "enter":
			if l.cancel {
				return l, core.Close(l)
			}
			l.id, l.busy = core.NextID(), true
			l.owner.logoutID = l.id
			st.Busy.Logout = true
			clear, id := st.Deps.ClearCredentials, l.id
			return l, func() tea.Msg {
				if err := clear(); err != nil {
					err = fmt.Errorf("logging out: %w", err)
					return logoutDoneMsg{id: id, err: err}
				}
				return logoutDoneMsg{id: id}
			}
		}
	}
	return l, nil
}

func (l *logoutModal) body(inner, limit, frame int) []string {
	p := ui.P()
	var lines []string
	for _, s := range ui.Wrap("Sync stops on this device. Your local keys stay.", inner) {
		lines = append(lines, ui.Paint(s, p.Muted))
	}
	if l.busy {
		lines = []string{ui.SpinnerGlyph(frame) + " " + ui.Paint("Logging out", p.Text)}
	}
	out, can := ui.Button("Log out", ui.DangerButton), ui.Button("Cancel", ui.Secondary)
	if l.cancel {
		out, can = ui.Button("Log out", ui.Secondary), ui.Button("Cancel", ui.Primary)
	}
	btns := widget.Buttons(inner, out, can)
	all := append(append(append([]string{""}, lines...), ""), btns...)
	if len(all) > limit {
		all = append(lines, btns...)
	}
	if len(all) > limit {
		all = append(all[:min(max(0, limit-len(btns)), len(lines))], btns...)
	}
	return all[:min(len(all), max(0, limit))]
}

func (l *logoutModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 50, maxW, maxH, l.body)
}

func (l *logoutModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Log out of Forged?", "", l.body)
}
