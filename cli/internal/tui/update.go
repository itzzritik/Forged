package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

// ErrUpdateRequested ends the TUI so the npm launcher can update and reopen it.
var ErrUpdateRequested = errors.New("update requested")

type updateRequestedMsg struct{}

type updateModal struct{ later bool }

func (a *app) offerUpdate() {
	if a.life.updateOffered || a.inGate || a.st.Locked || !a.st.UpdateReady() {
		return
	}
	a.life.updateOffered = true
	a.openUnder(&updateModal{})
}

func (a *app) requestUpdate() tea.Cmd {
	if a.st.Busy.PasswordChange {
		return a.quit()
	}
	a.updateRequested = true
	return tea.Quit
}

func (m *updateModal) Actions(st *core.State) []core.Action {
	if !st.Deps.ViaLauncher {
		return []core.Action{{Key: "enter", Label: "OK"}, {Key: "esc", Label: "Close"}}
	}
	label := "Update"
	if m.later {
		label = "Later"
	}
	return []core.Action{{Key: "enter", Label: label}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Later"}}
}

func (m *updateModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "tab", "shift+tab", "left", "right":
		m.later = !m.later
	case "esc":
		return m, core.Close(m)
	case "enter":
		if !st.Deps.ViaLauncher || m.later {
			return m, core.Close(m)
		}
		return m, tea.Batch(core.Close(m), core.Send(updateRequestedMsg{}))
	}
	return m, nil
}

func (m *updateModal) body(st *core.State) func(inner, limit, frame int) []string {
	return func(inner, limit, _ int) []string {
		p := ui.P()
		text, command := "forged will close, update with npm, and reopen. The vault locks during the restart, so you'll unlock again.", ""
		btns := widget.Buttons(inner, ui.Button("OK", ui.Primary), "")
		if st.Deps.ViaLauncher {
			update, later := ui.Button("Update", ui.Primary), ui.Button("Later", ui.Secondary)
			if m.later {
				update, later = ui.Button("Update", ui.Secondary), ui.Button("Later", ui.Primary)
			}
			btns = widget.Buttons(inner, update, later)
		} else {
			text, command = "Run this to update:", "npm i -g forgedssh@latest"
		}
		lines := []string{"", ui.VersionStep(st.Deps.AppVersion, st.LatestVersion), ""}
		for _, s := range ui.Wrap(text, inner) {
			lines = append(lines, ui.Paint(s, p.Muted))
		}
		if command != "" {
			lines = append(lines, ui.Paint(command, p.Text))
		}
		lines = append(append(lines, ""), btns...)
		if len(lines) > limit {
			lines = append(lines[:max(0, limit-len(btns))], btns...)
		}
		return lines[:min(len(lines), max(0, limit))]
	}
}

func (m *updateModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, m.body(st))
}

func (m *updateModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Update available", "", m.body(st))
}
