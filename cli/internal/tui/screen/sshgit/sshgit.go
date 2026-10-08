package sshgit

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	rowToggle  = 0
	rowSigning = 1
	rowRoutes  = 2
)

type toggleDoneMsg struct {
	id  core.ID
	err error
}

type Model struct {
	sel      int
	started  bool
	hidden   bool
	rec      bool
	confirm  bool
	routes   *routesModal
	debug    actions.SSHRoutingDebug
	loaded   bool
	loadErr  string
	reqID    core.ID
	toggleID core.ID
	signID   core.ID
	signBusy bool
}

func New() core.Screen { return &Model{} }

func (m *Model) Spinning() bool {
	if m.rec {
		return false
	}
	return m.signBusy || m.toggleID != 0
}

func routing() bool { return platform.SSHRoutingSupported() }

type cell struct {
	x int
	s string
}

func at(rx int, s string) cell { return cell{rx - ui.Width(s), s} }

func build(origin int, cells ...cell) string {
	var b strings.Builder
	cur := origin
	for _, c := range cells {
		if c.s == "" {
			continue
		}
		if c.x > cur {
			b.WriteString(ui.Repeat(" ", c.x-cur))
			cur = c.x
		}
		b.WriteString(c.s)
		cur += ui.Width(c.s)
	}
	return b.String()
}

func (m *Model) rows(st *core.State) int {
	switch {
	case st.Recovery():
		return 1
	case routing():
		return rowRoutes + 1
	}
	return rowRoutes
}

func (m *Model) clamp(st *core.State) {
	m.sel = m.selIdx(st)
}

func (m *Model) selIdx(st *core.State) int { return max(0, min(m.sel, m.rows(st)-1)) }

func (m *Model) canForgetAll(st *core.State) bool {
	return routing() && !st.Recovery() && (len(m.debug.Routes) > 0 || m.debug.RuntimeGuardRequired)
}

func (m *Model) Actions(st *core.State) []core.Action {
	if st.Recovery() {
		if st.Snapshot.AgentDisabled {
			return nil
		}
		return []core.Action{{Key: "enter", Label: "Turn off"}}
	}
	if m.selIdx(st) == rowRoutes {
		return []core.Action{{Key: "enter", Label: "Open"}}
	}
	return []core.Action{{Key: "enter", Label: "Change"}}
}

func (m *Model) reload(st *core.State) tea.Cmd {
	if st.Recovery() || !routing() {
		return nil
	}
	m.reqID = core.NextID()
	return loadCmd(st, m.reqID)
}

func (m *Model) polling() bool { return m.routes != nil && !m.confirm }

func (m *Model) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case core.SwitchTabMsg:
		was := m.hidden
		m.hidden = msg.Tab != core.TabSSH
		if was && !m.hidden {
			cmds = append(cmds, m.reload(st))
		}
	case core.RoutesMsg:
		if msg.ID != m.reqID {
			break
		}
		if msg.Err != nil {
			m.loadErr = st.Reporter.Report("ssh", "load routes", msg.Err)
		} else {
			m.loadErr, m.loaded, m.debug = "", true, msg.Debug
		}
		if m.routes != nil {
			m.routes.scroll(st)
		}
		if m.polling() {
			cmds = append(cmds, pollCmd(m.reqID))
		}
	case pollMsg:
		if msg.id == m.reqID && m.polling() && !st.Recovery() {
			cmds = append(cmds, loadCmd(st, m.reqID))
		}
	case toggleDoneMsg:
		if msg.id != m.toggleID {
			break
		}
		m.toggleID = 0
		st.Busy.SSHToggle = false
		if msg.err != nil {
			cmds = append(cmds, core.Toast(st.Reporter.Report("ssh", "change SSH integration", msg.err), ui.ToneBad))
		} else {
			cmds = append(cmds, core.Send(core.RefreshMsg{}))
		}
	case signChoiceMsg:
		cmds = append(cmds, m.changeSigning(st, msg))
	case core.SigningMsg:
		if msg.ID != m.signID || !m.signBusy {
			break
		}
		m.signBusy = false
		switch {
		case msg.Err != nil:
			cmds = append(cmds, core.Toast(st.Reporter.Report("ssh", "change commit signing", msg.Err), ui.ToneBad))
		case msg.Status.Mode == actions.CommitSigningForged:
			cmds = append(cmds, core.Toast("Signing with "+ui.Sanitize(msg.Status.KeyName), ui.ToneGood))
		default:
			cmds = append(cmds, core.Toast("Signing turned off", ui.ToneGood))
		}
	case forgetDoneMsg:
		if msg.err == nil && m.confirm {
			m.confirm = false
			cmds = append(cmds, m.reload(st))
		}
	case confirmClosedMsg:
		if m.confirm {
			m.confirm = false
			cmds = append(cmds, m.reload(st))
		}
	case core.LockedMsg:
		m.routes, m.confirm = nil, false
	case tea.KeyPressMsg:
		cmds = append(cmds, m.key(msg, st))
	}
	if rec := st.Recovery(); m.started && m.rec && !rec && !m.hidden {
		cmds = append(cmds, m.reload(st))
	}
	m.rec = st.Recovery()
	if !m.started {
		m.started = true
		if !m.hidden {
			cmds = append(cmds, m.reload(st))
		}
	}
	m.clamp(st)
	return m, tea.Batch(cmds...)
}

func (m *Model) key(msg tea.KeyPressMsg, st *core.State) tea.Cmd {
	switch strings.ToLower(msg.String()) {
	case "up", "k":
		m.sel = max(0, m.sel-1)
	case "down", "j":
		m.sel = min(m.sel+1, m.rows(st)-1)
	case "enter":
		return m.activate(st)
	}
	return nil
}

func (m *Model) openConfirm(st *core.State, c *confirmModal) tea.Cmd {
	m.confirm = true
	return widget.OpenModal(st, c)
}

func (m *Model) activate(st *core.State) tea.Cmd {
	switch {
	case m.sel == rowToggle:
		return m.toggle(st)
	case m.sel == rowSigning && !st.Recovery():
		return m.openDropdown(st)
	case m.sel == rowRoutes && !st.Recovery() && routing():
		m.routes, m.loadErr = &routesModal{m: m}, ""
		w, h := core.BodySize(st)
		return tea.Batch(m.reload(st), core.Send(core.OpenOverlayMsg{Screen: m.routes, W: w, H: h, Dim: true}))
	}
	return nil
}

func (m *Model) toggle(st *core.State) tea.Cmd {
	if st.Busy.Maintenance || st.Busy.SSHToggle || (st.Recovery() && st.Snapshot.AgentDisabled) {
		return nil
	}
	enable, disable := st.Deps.EnableSSHAgent, st.Deps.DisableSSHAgent
	off := st.Recovery() || st.Snapshot.ManagedSSHIntegration
	m.toggleID = core.NextID()
	st.Busy.SSHToggle = true
	id := m.toggleID
	return func() tea.Msg {
		var err error
		if off {
			err = disable()
		} else {
			err = enable()
		}
		if err != nil {
			err = fmt.Errorf("changing SSH integration: %w", err)
		}
		return toggleDoneMsg{id: id, err: err}
	}
}

func (m *Model) changeSigning(st *core.State, c signChoiceMsg) tea.Cmd {
	if m.signBusy {
		return nil
	}
	m.signID, m.signBusy = core.NextID(), true
	id := m.signID
	enable, disable := st.Deps.EnableCommitSigning, st.Deps.DisableCommitSigning
	return func() tea.Msg {
		var status actions.CommitSigningStatus
		var err error
		if c.off {
			status, err = disable()
		} else {
			status, err = enable(c.name)
		}
		if err != nil {
			err = fmt.Errorf("changing commit signing: %w", err)
		}
		return core.SigningMsg{ID: id, Status: status, Err: err}
	}
}

func (m *Model) signingValue(st *core.State) string {
	if !st.SigningLoaded {
		return "Checking"
	}
	switch st.Signing.Mode {
	case actions.CommitSigningForged:
		return ui.Sanitize(st.Signing.KeyName)
	case actions.CommitSigningExternal:
		if p := strings.TrimSpace(st.Signing.Program); p != "" {
			return "External (" + ui.Sanitize(filepath.Base(p)) + ")"
		}
		return "External"
	}
	return "Off"
}

func (m *Model) toggleValue(st *core.State) string {
	p := ui.P()
	switch {
	case m.toggleID != 0:
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Updating", p.Accent)
	case st.Recovery() && st.Snapshot.AgentDisabled:
		return ui.Paint(ui.G.Ring+" Off", p.Muted)
	case st.Recovery():
		return ui.Paint(ui.G.Next, p.Muted)
	case st.Snapshot.RequiresManualSSHConfigurationChange():
		return ui.Paint(ui.G.Warn+" External", p.Warn)
	case st.Snapshot.ManagedSSHIntegration:
		return ui.Paint(ui.G.Dot, p.Good) + " " + ui.Paint("On", p.Text)
	}
	return ui.Paint(ui.G.Ring+" Off", p.Muted)
}

func (m *Model) signingRowValue(st *core.State, f widget.Frame) string {
	p := ui.P()
	if m.signBusy {
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Updating signing", p.Accent)
	}
	v := ui.Trunc(m.signingValue(st), max(6, f.W/2))
	return ui.Paint(v+" "+ui.G.Caret, p.Text)
}

func (m *Model) signingY(st *core.State) int {
	f := widget.FrameFor(st, st.Width-4)
	y := 2
	if st.Snapshot.RequiresManualSSHConfigurationChange() {
		y++
	}
	return y + f.Gap + 1
}

func (m *Model) View(st *core.State, w, h int) string {
	sel := m.selIdx(st)
	p := ui.P()
	f := widget.FrameFor(st, w)
	section := func(s string) string { return f.Pad(ui.Paint(s, p.Muted)) }
	lines := []string{section("SSH agent"), f.Pad(ui.Row(f.W, ui.G.Icon.SSH, m.toggleLabel(st), m.toggleValue(st), sel == rowToggle))}
	if st.Recovery() {
		lines = append(lines, "")
		if st.Snapshot.AgentDisabled {
			lines = append(lines, f.Pad(ui.Paint(ui.Trunc("Forged will not manage SSH until its service is verified again.", f.W-2), p.Muted)))
		} else {
			lines = append(lines, f.Pad(ui.Paint(ui.Trunc("Forged cannot verify its background service.", f.W-2), p.Muted)))
		}
		return fit(lines, w, h)
	}
	if st.Snapshot.RequiresManualSSHConfigurationChange() {
		lines = append(lines, f.Pad(ui.Banner(f.W, 1, ui.ToneWarn, "Forged stays active through your own SSH config. Update it by hand to turn it off.", "", "")))
	}
	for range f.Gap {
		lines = append(lines, "")
	}
	lines = append(lines, section("Git commit signing"), f.Pad(ui.Row(f.W, ui.G.Icon.Sign, "Sign commits with", m.signingRowValue(st, f), sel == rowSigning)))
	if routing() {
		for range f.Gap {
			lines = append(lines, "")
		}
		lines = append(lines, f.Pad(ui.Row(f.W, ui.G.Icon.Routes, "Learned routes", m.routesValue(f), sel == rowRoutes)))
	}
	return fit(lines, w, h)
}

func (m *Model) routesValue(f widget.Frame) string {
	p := ui.P()
	v, c := core.Plural(len(m.debug.Routes), "route")+" "+ui.G.Next, p.Text
	switch {
	case !m.loaded && m.loadErr != "":
		v, c = "Couldn't load", p.Danger
	case !m.loaded:
		v, c = "Loading", p.Muted
	case m.debug.RuntimeGuardRequired:
		v, c = ui.G.Warn+" SSH route guard needs a reset", p.Warn
	case len(m.debug.Routes) == 0:
		v, c = "None", p.Muted
	}
	return ui.Paint(ui.Trunc(v, max(6, f.W/2)), c)
}

func (m *Model) toggleLabel(st *core.State) string {
	switch {
	case st.Recovery() && st.Snapshot.AgentDisabled:
		return "SSH integration is off"
	case st.Recovery():
		return "Turn off SSH integration"
	}
	return "Use Forged for SSH"
}

func fit(lines []string, w, h int) string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	lines = lines[:max(0, h)]
	for i, l := range lines {
		lines[i] = ui.Pad(ui.Trunc(l, w), w)
	}
	return ui.Join(lines)
}
