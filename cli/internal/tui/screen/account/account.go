package account

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type rowKind int

const (
	rowLogin rowKind = iota
	rowSync
	rowInterval
	rowHeadless
	rowPassword
	rowLogout
)

type intervalChoiceMsg struct{ value string }

type intervalDoneMsg struct {
	id  core.ID
	err error
}

type Model struct {
	sel        rowKind
	saving     bool
	savingID   core.ID
	savingVal  string
	pwID       core.ID
	logoutID   core.ID
	headlessID core.ID
	spinning   bool
}

func New() core.Screen { return &Model{sel: rowLogin} }

func (m *Model) Spinning() bool { return m.spinning }

func refused(st *core.State) bool {
	return st.Busy.Maintenance || st.Busy.Sync || st.Busy.Logout || st.Busy.PasswordChange
}

func signedIn(st *core.State) bool { return st.Snapshot.LoggedIn && st.CredentialError() == "" }

func rows(st *core.State) []rowKind {
	out := []rowKind{rowLogin, rowInterval}
	if signedIn(st) {
		out[0] = rowSync
	}
	if st.SecurityLoaded && st.Security.HeadlessSupported {
		out = append(out, rowHeadless)
	}
	out = append(out, rowPassword)
	if signedIn(st) {
		out = append(out, rowLogout)
	}
	return out
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
		out = append(out, core.Action{Key: "h", Label: "Open Health", Icon: ui.G.Icon.Health})
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
	case headlessDoneMsg:
		cmd = m.headlessDone(st, msg)
	case tea.KeyPressMsg:
		cmd = m.key(msg, st)
	}
	m.sel = m.cur(st)
	m.spinning = m.saving || m.headlessID != 0 || st.Status.Syncing || st.Busy.Logout
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
	case rowLogin:
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
	case rowHeadless:
		switch {
		case m.headlessID != 0:
		case st.Security.HeadlessUnlock:
			return m.setHeadless(st, false, core.NextID())
		default:
			return widget.OpenModal(st, &headlessModal{owner: m})
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

func (m *Model) setHeadless(st *core.State, enable bool, id core.ID) tea.Cmd {
	m.headlessID = id
	on, off := st.Deps.EnableHeadlessUnlock, st.Deps.DisableHeadlessUnlock
	return func() tea.Msg {
		if !enable {
			if err := off(); err != nil {
				return headlessDoneMsg{id: id, err: fmt.Errorf("turning off automatic unlock: %w", err)}
			}
			return headlessDoneMsg{id: id}
		}
		enrolled, err := on()
		if err != nil {
			err = fmt.Errorf("turning on automatic unlock: %w", err)
		}
		return headlessDoneMsg{id: id, enable: true, later: !enrolled, err: err}
	}
}

func (m *Model) headlessDone(st *core.State, msg headlessDoneMsg) tea.Cmd {
	if msg.skip {
		if msg.err != nil {
			return core.Toast(st.Reporter.Report("account", "skip automatic unlock", msg.err), ui.ToneBad)
		}
		return nil
	}
	if msg.id != m.headlessID {
		return nil
	}
	m.headlessID = 0
	reload := core.LoadSecurityCmd(st)
	switch {
	case msg.err != nil:
		return tea.Batch(reload, core.Toast(st.Reporter.Report("account", "set automatic unlock", msg.err), ui.ToneBad))
	case !msg.enable:
		st.Security.HeadlessUnlock = false
		return tea.Batch(reload, core.Toast("Automatic unlock is off", ui.ToneGood))
	}
	st.Security.HeadlessUnlock, st.Security.HeadlessOffered = true, true
	if msg.later {
		return tea.Batch(reload, core.Toast("Takes effect at your next unlock", ui.ToneWarn))
	}
	return tea.Batch(reload, core.Toast("Automatic unlock is on", ui.ToneGood))
}

func (m *Model) headlessValue(st *core.State) (string, bool) {
	switch {
	case m.headlessID != 0:
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Saving", ui.P().Accent), true
	case st.Security.HeadlessUnlock:
		return "On", false
	}
	return "Off", false
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

func syncStatus(st *core.State, w int) string {
	p := ui.P()
	switch {
	case st.SyncIssue() != "":
		return ui.Paint(ui.Trunc("Sync needs attention", w), p.Danger)
	case st.Status.Syncing:
		return ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(ui.Trunc(" Syncing", w-1), p.Accent)
	}
	t := st.Status.LastSuccessfulPullAt
	if p := st.Status.LastSuccessfulPushAt; p.After(t) {
		t = p
	}
	if t.IsZero() {
		return ui.Paint(ui.Trunc("Not synced yet", w), p.Muted)
	}
	return ui.Paint(ui.Trunc("Synced "+core.Ago(t.UTC().Format(time.RFC3339), time.Now()), w), p.Muted)
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
	row := func(k rowKind, icon, label, value string, c func() (string, bool), muted bool) {
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
		add(f.Pad(ui.Row(f.W, icon, label, ui.Paint(value, color), sel == k)))
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
	if text != "" && f.Gap > 0 {
		add("")
	}

	if signedIn(st) {
		name := ui.Sanitize(strings.TrimSpace(st.AccountName))
		email := ui.Sanitize(strings.TrimSpace(st.AccountEmail))
		if name == "" {
			name = core.FallbackName(email)
		}
		tile := avatar(initials(name))
		if name == "" {
			name = "Signed in"
		}
		tw := f.W - ui.Width(tile[0])
		for i, t := range []string{ui.Bold(ui.Trunc(name, tw), p.Text), ui.Paint(ui.Trunc(email, tw), p.Muted), syncStatus(st, tw)} {
			add(f.Pad(tile[i] + t))
		}
		gap()
		section("Sync")
		row(rowSync, ui.G.Icon.Sync, "Sync now", "", nil, true)
	} else {
		heading := "Sync is off"
		if st.Snapshot.LoggedIn {
			heading = "Sync is paused"
		}
		add(f.Pad(ui.Bold(heading, p.Text)))
		gap()
		row(rowLogin, ui.G.Icon.Login, "Log in", ui.G.Next, nil, true)
		if !st.Snapshot.LoggedIn && st.Width >= 60 {
			lead := 2 + ui.IconWidth(ui.G.Icon.Login)
			add(f.Pad(ui.Repeat(" ", lead) + ui.Paint(ui.Trunc("Sync encrypted keys across your machines", f.W-lead), p.Muted)))
		}
	}
	gap()
	section("Security")
	row(rowInterval, ui.G.Icon.Clock, "Ask for master password every", "", func() (string, bool) { return m.intervalValue(st) }, false)
	if slices.Contains(rows(st), rowHeadless) {
		row(rowHeadless, ui.G.Icon.Lock, "Unlock automatically on this server", "", func() (string, bool) { return m.headlessValue(st) }, false)
	}
	row(rowPassword, ui.G.Icon.Secret, "Change master password", ui.G.Next, nil, true)
	if signedIn(st) {
		row(rowLogout, ui.G.Icon.Logout, "Log out", ui.G.Next, nil, true)
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

func initials(name string) string {
	var out []rune
	words := strings.Fields(name)
	for i, w := range words {
		if i > 0 && i < len(words)-1 {
			continue
		}
		for _, r := range w {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if ui.Width(string(append(out, r))) <= 2 {
					out = append(out, unicode.ToUpper(r))
				}
				break
			}
		}
	}
	return string(out)
}

func avatar(ini string) [3]string {
	if ini == "" {
		return [3]string{}
	}
	p := ui.P()
	fill := ui.Fg(p.OnAccent).Background(p.Accent).Bold(true)
	pad := 6 - ui.Width(ini)
	blank := fill.Render(ui.Repeat(" ", 6)) + "  "
	return [3]string{blank, fill.Render(ui.Repeat(" ", pad/2)+ini+ui.Repeat(" ", pad-pad/2)) + "  ", blank}
}
