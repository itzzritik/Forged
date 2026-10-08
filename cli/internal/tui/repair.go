package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type repairCancelMsg struct{}

type repairModal struct {
	sec    ui.Secret
	errMsg string
}

func (a *app) openRepair(errText string) tea.Cmd {
	m := &repairModal{sec: ui.NewSecret(128), errMsg: errText}
	w, h := m.Size(a.st, a.st.Width-4, a.st.Height-2)
	a.overlays = append(a.overlays, core.OpenOverlayMsg{Screen: m, W: w, H: h, Center: true, Dim: true})
	if a.active != core.TabHealth {
		return a.setTab(core.TabHealth)
	}
	return nil
}

func (m *repairModal) Actions(*core.State) []core.Action {
	return []core.Action{{Key: "enter", Label: "Fix issues"}, {Key: "esc", Label: "Cancel"}}
}

func (m *repairModal) Update(msg tea.Msg, _ *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case core.LockedMsg:
		m.sec.Wipe()
	case tea.PasteMsg:
		m.errMsg = ""
		m.sec.Update(msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.sec.Wipe()
			return m, tea.Sequence(core.Close(m), core.Send(repairCancelMsg{}))
		case "enter":
			if m.sec.Len() == 0 {
				m.errMsg = "Enter your master password"
				return m, nil
			}
			return m, tea.Sequence(core.Close(m), core.Send(core.RequestRepairMsg{Password: m.sec.Take()}))
		}
		if m.sec.Update(msg) {
			m.errMsg = ""
		}
	}
	return m, nil
}

func (m *repairModal) body(inner, limit, _ int) []string {
	p := ui.P()
	errLine := ""
	if m.errMsg != "" {
		errLine = ui.Paint(ui.Trunc(ui.Sanitize(m.errMsg), inner), p.Danger)
	}
	lines := []string{ui.Paint(ui.Trunc("Master password", inner), p.Muted), m.sec.View(inner, true), errLine}
	text := ui.Wrap("Enter your master password to repair this machine and restore secure access.", inner)
	if len(text)+1+len(lines) <= limit {
		for i, l := range text {
			text[i] = ui.Paint(l, p.Text)
		}
		lines = append(append(text, ""), lines...)
	}
	if len(lines)+1 <= limit {
		lines = append([]string{""}, lines...)
	}
	return lines[:min(len(lines), max(0, limit))]
}

func (m *repairModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, m.body)
}

func (m *repairModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Fix issues", "", m.body)
}
