package account

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type rowKind int

const (
	rowRepair rowKind = iota
	rowLogin
	rowSync
	rowInterval
	rowPassword
	rowLogout
)

type intervalChoiceMsg struct{ value string }

type intervalDoneMsg struct {
	id  core.ID
	err error
}

type Model struct {
	sel       rowKind
	saving    bool
	savingID  core.ID
	savingVal string
	pwID      core.ID
	logoutID  core.ID
	spinning  bool
}

func New() core.Screen { return &Model{sel: rowRepair} }

func (m *Model) Spinning() bool { return m.spinning }

func refused(st *core.State) bool {
	return st.Busy.Maintenance || st.Busy.Sync || st.Busy.Logout || st.Busy.PasswordChange
}

func rows(st *core.State) []rowKind {
	var out []rowKind
	if st.CredentialError() != "" {
		out = append(out, rowRepair)
	}
	if st.Snapshot.LoggedIn {
		return append(out, rowSync, rowInterval, rowPassword, rowLogout)
	}
	return append(out, rowLogin, rowInterval, rowPassword)
}

func (m *Model) cur(st *core.State) rowKind {
	r := rows(st)
	if slices.Contains(r, m.sel) {
		return m.sel
	}
	return r[0]
}

func (m *Model) Actions(st *core.State) []core.Action {
	out := []core.Action{{Key: "enter", Label: "Open"}, {Key: ui.G.UpDown, Label: "Select"}}
	if st.SyncIssue() != "" {
		out = append(out, core.Action{Key: "h", Label: "Open Health"})
	}
	return out
}

func (m *Model) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case intervalChoiceMsg:
		cmd = m.setInterval(st, msg.value)
	case intervalDoneMsg:
		if msg.id != m.savingID || !m.saving {
			break
		}
		m.saving = false
		if msg.err != nil {
			cmd = core.Toast(st.Reporter.Report("account", "set master password interval", msg.err), ui.ToneBad)
		} else {
			st.Security.MasterPasswordInterval = m.savingVal
			cmd = tea.Batch(core.Toast("Saved", ui.ToneGood), core.LoadSecurityCmd(st))
		}
	case passwordDoneMsg:
		if msg.id != m.pwID || !st.Busy.PasswordChange {
			break
		}
		st.Busy.PasswordChange = false
		switch detail := ui.Sanitize(strings.TrimSpace(msg.detail)); {
		case msg.err != nil:
			cmd = core.Toast(st.Reporter.Report("account", "change master password", msg.err), ui.ToneBad)
		case detail != "":
			cmd = core.Toast(detail, ui.ToneWarn)
		default:
			cmd = core.Toast("Master password changed", ui.ToneGood)
		}
	case logoutDoneMsg:
		if msg.id != m.logoutID || !st.Busy.Logout {
			break
		}
		cmd = m.finishLogout(st, msg.err)
	case tea.KeyPressMsg:
		cmd = m.key(msg, st)
	}
	m.sel = m.cur(st)
	m.spinning = m.saving || st.Status.Syncing || st.Busy.Logout
	return m, cmd
}

func (m *Model) move(st *core.State, d int) {
	r := rows(st)
	i := slices.Index(r, m.cur(st)) + d
	m.sel = r[max(0, min(i, len(r)-1))]
}

func (m *Model) key(msg tea.KeyPressMsg, st *core.State) tea.Cmd {
	switch strings.ToLower(msg.String()) {
	case "up", "k":
		m.move(st, -1)
	case "down", "j":
		m.move(st, 1)
	case "h":
		if st.SyncIssue() != "" {
			return core.Send(core.SwitchTabMsg{Tab: core.TabHealth})
		}
	case "enter":
		if refused(st) {
			return nil
		}
		return m.activate(st)
	}
	return nil
}

func (m *Model) activate(st *core.State) tea.Cmd {
	switch m.cur(st) {
	case rowRepair, rowLogin:
		return core.Send(core.RequestLoginMsg{})
	case rowSync:
		return core.Send(core.RequestSyncMsg{})
	case rowInterval:
		switch {
		case st.SecurityErr != "":
			return core.LoadSecurityCmd(st)
		case st.SecurityLoaded && !m.saving:
			return m.openMenu(st)
		}
	case rowPassword:
		return widget.OpenModal(st, newPasswordModal(m))
	case rowLogout:
		return widget.OpenModal(st, &logoutModal{owner: m})
	}
	return nil
}

func (m *Model) finishLogout(st *core.State, err error) tea.Cmd {
	st.Busy.Logout = false
	warning := logoutWarning(err)
	if err != nil && warning == "" {
		return core.Toast(st.Reporter.Report("account", "log out", err), ui.ToneBad)
	}
	st.AccountName, st.AccountEmail = "", ""
	st.Snapshot.LoggedIn, st.Snapshot.LoginCheckError = false, ""
	st.Status.Linked, st.Status.Syncing, st.Status.Error = false, false, ""
	st.Status.LastSuccessfulPullAt, st.Status.LastSuccessfulPushAt = time.Time{}, time.Time{}
	done := core.Toast("Logged out", ui.ToneGood)
	if warning != "" {
		done = core.Toast(warning, ui.ToneWarn)
	}
	return tea.Batch(core.Send(core.RefreshMsg{}), done)
}

func (m *Model) setInterval(st *core.State, v string) tea.Cmd {
	if m.saving || refused(st) {
		return nil
	}
	m.saving, m.savingID, m.savingVal = true, core.NextID(), v
	set, id := st.Deps.SetMasterPasswordInterval, m.savingID
	return func() tea.Msg {
		err := set(v)
		if err != nil {
			err = fmt.Errorf("saving master password interval: %w", err)
		}
		return intervalDoneMsg{id: id, err: err}
	}
}

func (m *Model) syncValue(st *core.State, f widget.Frame) (string, bool) {
	if st.Status.Syncing {
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Syncing", ui.P().Accent), true
	}
	t := st.Status.LastSuccessfulPullAt
	if p := st.Status.LastSuccessfulPushAt; p.After(t) {
		t = p
	}
	if t.IsZero() {
		return "Not yet synced", false
	}
	ago := core.Ago(t.UTC().Format(time.RFC3339), time.Now())
	if f.W >= 40 {
		return "Synced " + ago, false
	}
	return strings.ToUpper(ago[:1]) + ago[1:], false
}

func (m *Model) intervalValue(st *core.State) (string, bool) {
	switch {
	case m.saving:
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Saving", ui.P().Accent), true
	case st.SecurityErr != "":
		return "Couldn't load, press enter to retry", false
	case !st.SecurityLoaded:
		return "Loading", false
	}
	return core.IntervalLabel(st.Security.MasterPasswordInterval) + " " + ui.G.Caret, false
}

type layout struct {
	lines []string
	y     map[rowKind]int
}

func (m *Model) build(st *core.State, w int) layout {
	p := ui.P()
	f := widget.FrameFor(st, w)
	v := layout{y: map[rowKind]int{}}
	add := func(s string) { v.lines = append(v.lines, s) }
	gap := func() {
		for range f.Gap {
			add("")
		}
	}
	section := func(s string) { add(f.Pad(ui.Paint(s, p.Muted))) }
	sel := m.cur(st)
	row := func(k rowKind, label, value string, c func() (string, bool), muted bool) {
		color := p.Text
		if muted {
			color = p.Muted
		}
		if c != nil {
			var accent bool
			if value, accent = c(); accent {
				color = p.Accent
			}
		}
		value = ui.Trunc(value, max(6, f.W/2))
		v.y[k] = len(v.lines)
		add(f.Pad(ui.Row(f.W, label, value, sel == k, color)))
	}

	text, tone, key := m.banner(st)
	if text != "" {
		bh := 1
		if st.Height >= 24 {
			bh = 3
		}
		for _, l := range strings.Split(ui.Banner(f.W, bh, tone, text, key, "Open Health"), "\n") {
			add(f.Pad(l))
		}
	}
	if slices.Contains(rows(st), rowRepair) {
		row(rowRepair, "Repair account", ui.G.Next, nil, true)
	}
	if text != "" && f.Gap > 0 {
		add("")
	}

	if st.Snapshot.LoggedIn {
		name := ui.Sanitize(strings.TrimSpace(st.AccountName))
		email := ui.Sanitize(strings.TrimSpace(st.AccountEmail))
		if name == "" {
			name = core.FallbackName(email)
		}
		if name == "" {
			name = "Signed in"
		}
		add(f.Pad(ui.Bold(ui.Trunc(name, f.W), p.Text)))
		add(f.Pad(ui.Paint(ui.Trunc(email, f.W), p.Muted)))
		gap()
		section("Sync")
		row(rowSync, "Sync now", "", func() (string, bool) { return m.syncValue(st, f) }, true)
	} else {
		add(f.Pad(ui.Bold("Sync is off", p.Text)))
		gap()
		row(rowLogin, "Log in", ui.G.Next, nil, true)
		if st.Width >= 60 {
			add(f.Pad("  " + ui.Paint(ui.Trunc("Sync encrypted keys across your machines", f.W-2), p.Muted)))
		}
	}
	gap()
	section("Security")
	row(rowInterval, "Ask for master password every", "", func() (string, bool) { return m.intervalValue(st) }, false)
	row(rowPassword, "Change master password", ui.G.Next, nil, true)
	if st.Snapshot.LoggedIn {
		row(rowLogout, "Log out", ui.G.Next, nil, true)
	}
	return v
}

func (m *Model) banner(st *core.State) (text string, tone ui.Tone, key string) {
	if issue := st.SyncIssue(); issue != "" {
		return "Sync needs attention: " + ui.Sanitize(issue), ui.ToneBad, "h"
	}
	if e := st.CredentialError(); e != "" {
		return ui.Sanitize(e), ui.ToneWarn, ""
	}
	return "", 0, ""
}

func (m *Model) View(st *core.State, w, h int) string {
	v := m.build(st, w)
	top := max(0, v.y[m.cur(st)]-h+1)
	lines := v.lines[min(top, len(v.lines)):]
	for len(lines) < h {
		lines = append(lines, "")
	}
	lines = lines[:max(0, h)]
	for i, l := range lines {
		lines[i] = ui.Pad(ui.Trunc(l, w), w)
	}
	return ui.Join(lines)
}

var intervals = []string{config.MasterPasswordInterval7Days, config.MasterPasswordInterval15Days, config.MasterPasswordInterval30Days}

func (m *Model) openMenu(st *core.State) tea.Cmd {
	bw, bh := core.BodySize(st)
	v := m.build(st, bw)
	y := v.y[rowInterval]
	labels := make([]string, len(intervals))
	for i, iv := range intervals {
		labels[i] = core.IntervalLabel(iv)
	}
	d := &widget.Dropdown{
		Items: labels, Cur: slices.Index(intervals, config.NormalizeMasterPasswordInterval(st.Security.MasterPasswordInterval)),
		Anchor: y - max(0, y-bh+1), MaxW: 22,
		Choose: func(i int) tea.Msg { return intervalChoiceMsg{value: intervals[i]} },
	}
	return d.Open(st)
}
