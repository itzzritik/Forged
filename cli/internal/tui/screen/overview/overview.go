package overview

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type item actions.KeySummary

func (i item) FilterValue() string { return i.Name }

type copyDoneMsg struct {
	since core.ID
	err   error
}

type model struct {
	list     list.Model
	delegate *delegate
	host     string
}

func New() core.Screen {
	d := &delegate{}
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	host, _, _ := strings.Cut(hostname(), ".")
	return &model{list: l, delegate: d, host: host}
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func bodyRows(st *core.State) int { _, h := core.BodySize(st); return h }

func bannerShown(st *core.State, h int) bool { return st.Problems() > 0 && h >= 6 }

func switchTab(tab core.Tab, key string) tea.Cmd {
	if key == "" {
		return core.Send(core.SwitchTabMsg{Tab: tab})
	}
	return tea.Sequence(core.Send(core.SwitchTabMsg{Tab: tab}), core.Send(core.Press(key)))
}

func (m *model) Actions(st *core.State) []core.Action {
	out := []core.Action{{Key: "enter", Label: "Open"}, {Key: "c", Label: "Copy public"}, {Key: "n", Label: "New key"}, {Key: "i", Label: "Import"}}
	if st.Snapshot.LoggedIn {
		out = append(out, core.Action{Key: "s", Label: "Sync now"})
	}
	return append(out, core.Action{Key: "l", Label: "Lock"})
}

func (m *model) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	m.syncItems(st)
	switch msg := msg.(type) {
	case copyDoneMsg:
		if msg.err != nil {
			text := st.Reporter.Report("overview", "copy public key", msg.err)
			return m, core.Toast(text, ui.ToneBad)
		}
		return m, tea.Sequence(core.Toast("Public key copied", ui.ToneGood), core.Send(core.CancelClipMsg{Since: msg.since}))
	case tea.KeyPressMsg:
		return m, m.key(strings.ToLower(msg.String()), st)
	}
	return m, nil
}

func (m *model) key(k string, st *core.State) tea.Cmd {
	switch k {
	case "up", "k":
		m.list.CursorUp()
	case "down", "j":
		m.list.CursorDown()
	case "enter":
		if it, ok := m.list.SelectedItem().(item); ok {
			return core.Send(core.SwitchTabMsg{Tab: core.TabKeys, Select: it.Name})
		}
	case "c":
		if it, ok := m.list.SelectedItem().(item); ok {
			return core.CopyPublic(st, it.Name, func(since core.ID, err error) tea.Msg { return copyDoneMsg{since, err} })
		}
	case "n", "i":
		return switchTab(core.TabKeys, k)
	case "s":
		if st.Snapshot.LoggedIn {
			return core.Send(core.RequestSyncMsg{})
		}
	case "f":
		if bannerShown(st, bodyRows(st)) {
			return switchTab(core.TabHealth, "f")
		}
	}
	return nil
}

func lastUsed(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func (m *model) syncItems(st *core.State) {
	keys := slices.Clone(st.Keys)
	slices.SortStableFunc(keys, func(a, b actions.KeySummary) int {
		if c := lastUsed(b.LastUsedAt).Compare(lastUsed(a.LastUsedAt)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	cur := m.list.Items()
	same := len(cur) == len(keys)
	for i := 0; same && i < len(keys); i++ {
		same = cur[i].(item) == item(keys[i])
	}
	if same {
		return
	}
	selected := ""
	if it, ok := m.list.SelectedItem().(item); ok {
		selected = it.Name
	}
	items := make([]list.Item, len(keys))
	for i, k := range keys {
		items[i] = item(k)
	}
	m.list.SetItems(items)
	if i := slices.IndexFunc(keys, func(k actions.KeySummary) bool { return k.Name == selected }); i >= 0 {
		m.list.Select(i)
	}
}

type card struct {
	value, detail, short, panel, line string
	dot                               color.Color
}

func keyCount(st *core.State) string {
	n := len(st.Keys)
	if !st.KeysLoaded {
		n = st.Snapshot.KeyCount
	}
	return core.Plural(n, "key")
}

func agentCard(st *core.State) card {
	p := ui.P()
	n := keyCount(st)
	switch {
	case st.Snapshot.RequiresManualSSHConfigurationChange():
		return card{"External", "Managed outside Forged", "Outside Forged", "External", "Agent external", p.Warn}
	case st.Snapshot.AgentDisabled:
		return card{"Off", "Turn on in SSH & Git", "Turn on in SSH", "Off", "Agent off", p.Warn}
	}
	return card{"Active", n + " available to SSH", n, "Active", "Agent active", p.Good}
}

func signingCard(st *core.State) card {
	p := ui.P()
	switch {
	case !st.SigningLoaded && strings.TrimSpace(st.SigningErr) == "":
		return card{"Checking", "Reading signing setup", "Checking", "Checking", "Signing checking", p.Accent}
	case strings.TrimSpace(st.SigningErr) != "":
		return card{"Issue", "Could not read signing", "Issue", "Issue", "Signing issue", p.Warn}
	}
	switch st.Signing.Mode {
	case actions.CommitSigningForged:
		name := ui.Sanitize(st.Signing.KeyName)
		return card{"On", "Signing with " + name, name, name, "Signing on", p.Good}
	case actions.CommitSigningExternal:
		who := "Another tool signs"
		if prog := strings.TrimSpace(st.Signing.Program); prog != "" {
			who = "Signed by " + ui.Sanitize(filepath.Base(prog))
		}
		return card{"External", who, "External", "External", "Signing external", p.Warn}
	}
	return card{"Off", "Pick a key in SSH & Git", "Pick in SSH", "Off", "Signing off", p.Faint}
}

func syncCard(st *core.State, now time.Time) card {
	p := ui.P()
	switch {
	case !st.Snapshot.LoggedIn:
		return card{"Local only", "Log in to sync keys", "Log in to sync", "Local only", "Local only", p.Faint}
	case st.StatusDown:
		return card{"Needs attention", "Daemon not responding", "Not responding", "Needs attention", "Needs attention", p.Danger}
	case st.SyncIssue() != "":
		issue := ui.Sanitize(st.SyncIssue())
		return card{"Needs attention", issue, issue, "Needs attention", "Needs attention", p.Danger}
	case st.CredentialError() != "":
		return card{"Needs attention", "Log in again", "Log in again", "Needs attention", "Needs attention", p.Warn}
	case st.SyncPending():
		return card{"Syncing", "Sending changes", "Syncing", "Syncing", "Syncing", p.Accent}
	case !st.StatusLoaded:
		return card{"Checking", "Reading sync status", "Checking", "Checking", "Checking sync", p.Accent}
	}
	at := st.Status.LastSuccessfulPullAt
	if st.Status.LastSuccessfulPushAt.After(at) {
		at = st.Status.LastSuccessfulPushAt
	}
	if at.IsZero() {
		return card{"Up to date", "Not synced yet", "Not synced yet", "Not synced yet", "Up to date", p.Good}
	}
	ago := core.Ago(at.UTC().Format(time.RFC3339), now)
	return card{"Up to date", "Last synced " + ago, ago, ago, "Synced " + ago, p.Good}
}

func dotLine(c color.Color, text string) string {
	return "  " + ui.Paint(ui.G.Dot, c) + " " + text
}

func (m *model) View(st *core.State, w, h int) string {
	m.syncItems(st)
	now := time.Now()
	var lines []string
	if bannerShown(st, h) {
		n := st.Problems()
		text := fmt.Sprintf("%d problems need attention", n)
		if n == 1 {
			text = "1 problem needs attention"
		}
		lines = append(lines, ui.Banner(w, 1, ui.ToneWarn, text, "f", "Fix"))
	}
	cards := []card{agentCard(st), signingCard(st), syncCard(st, now)}
	rw := 0
	switch cols, rows := st.Width, st.Height; {
	case cols >= 80 && rows >= 22:
		ch := 4
		if rows >= 28 {
			ch = 6
		}
		lines = append(lines, cardRow(cards, w, ch, &rw)...)
		lines = append(lines, "")
	case rows >= 16:
		lines = append(lines, statusPanel(cards, w)...)
		lines = append(lines, "")
	default:
		worst := cards[0].dot
		for _, c := range cards {
			if c.dot != ui.P().Good {
				worst = c.dot
				break
			}
		}
		lines = append(lines, ui.Paint(ui.G.Dot, worst)+" "+ui.Paint(ui.Trunc(cards[0].line+"   "+cards[1].line+"   "+cards[2].line, w-2), ui.P().Muted), "")
	}
	remain := h - len(lines)
	if remain >= 3 {
		lw := w
		if rw > 0 {
			lw = w - rw - 2
		}
		left := m.recentPanel(st, lw, remain, now)
		right := rightColumn(st, m.host, rw, remain)
		for i, l := range left {
			if rw > 0 && i < len(right) {
				l = ui.Pad(l, lw) + "  " + right[i]
			}
			lines = append(lines, l)
		}
	}
	for i, l := range lines {
		lines[i] = ui.Pad(ui.Trunc(l, w), w)
	}
	for len(lines) < h {
		lines = append(lines, ui.Repeat(" ", w))
	}
	return ui.Join(lines[:h])
}

func cardRow(cards []card, w, ch int, last *int) []string {
	base := (w - 4) / 3
	rem := w - 4 - base*3
	blocks := make([][]string, len(cards))
	var widths []int
	for i, c := range cards {
		cw := base
		if i >= 3-rem {
			cw++
		}
		widths = append(widths, cw)
		inner := cw - 6
		detail := c.detail
		if ui.Width(detail) > inner {
			detail = c.short
		}
		body := []string{dotLine(c.dot, ui.Bold(ui.Trunc(c.value, inner-2), ui.P().Text)), "  " + ui.Paint(ui.Trunc(detail, inner), ui.P().Muted)}
		if ch >= 6 {
			body = append([]string{""}, body...)
		}
		blocks[i] = strings.Split(ui.Panel(cw, ch, titleOf(i), "", false, body), "\n")
	}
	*last = widths[len(widths)-1]
	out := make([]string, ch)
	for r := range out {
		var parts []string
		for i, b := range blocks {
			parts = append(parts, ui.Pad(b[r], widths[i]))
		}
		out[r] = strings.Join(parts, "  ")
	}
	return out
}

func titleOf(i int) string { return [...]string{"SSH agent", "Commit signing", "Sync"}[i] }

func statusPanel(cards []card, w int) []string {
	labels := [...]string{"SSH agent", "Signing", "Synced"}
	inner := w - 2
	body := make([]string, len(cards))
	for i, c := range cards {
		v := ui.Trunc(c.panel, max(0, w-18))
		if i == 2 {
			v = ui.Trunc(c.short, max(0, w-18))
		}
		left := dotLine(c.dot, ui.Paint(labels[i], ui.P().Text))
		body[i] = ui.Pad(left, inner-2-ui.Width(v)) + ui.Paint(v, ui.P().Muted)
	}
	return strings.Split(ui.Panel(w, 5, "Status", "", false, body), "\n")
}

func rightColumn(st *core.State, host string, rw, remain int) []string {
	if rw <= 0 || remain < 3 {
		return nil
	}
	qa := min(6, remain)
	if remain >= 12 {
		qa = 8
	}
	if remain-qa-1 < 7 {
		qa = remain
	}
	out := strings.Split(quickPanel(st, rw, qa), "\n")
	if qa < remain {
		out = append(out, "")
		out = append(out, strings.Split(devicePanel(st, host, rw, remain-qa-1), "\n")...)
	}
	return out
}

func quickPanel(st *core.State, w, h int) string {
	var body []string
	if h >= 8 {
		body = append(body, "")
	}
	items := [][2]string{{"n", "New key"}, {"i", "Import keys"}}
	if st.Snapshot.LoggedIn {
		items = append(items, [2]string{"s", "Sync now"})
	}
	items = append(items, [2]string{"l", "Lock"})
	for _, it := range items {
		body = append(body, "  "+ui.Keycap(it[0])+"  "+ui.Paint(ui.Trunc(it[1], w-11), ui.P().Text))
	}
	return ui.Panel(w, h, "Quick actions", "", false, body)
}

func devicePanel(st *core.State, host string, w, h int) string {
	unlock := "Password"
	if st.SecurityLoaded && st.Security.SystemAuthCapability == "available" {
		unlock = map[string]string{"darwin": "Touch ID", "windows": "Windows Hello"}[runtime.GOOS]
		if unlock == "" {
			unlock = "System auth"
		}
	}
	interval := "-"
	if st.SecurityLoaded {
		interval = "every " + core.IntervalLabel(st.Security.MasterPasswordInterval)
	}
	rows := [][2]string{{"Name", host}, {"Version", st.Deps.AppVersion}, {"Unlock", unlock}, {"Password", interval}}
	body := []string{""}
	for _, r := range rows {
		body = append(body, "  "+ui.Paint(ui.Pad(r[0], 10), ui.P().Muted)+ui.Paint(ui.Trunc(ui.Sanitize(r[1]), w-16), ui.P().Text))
	}
	return ui.Panel(w, h, "This device", "", false, body)
}

func (m *model) recentPanel(st *core.State, w, h int, now time.Time) []string {
	pad := 0
	if h >= 10 {
		pad = 1
	}
	n := max(0, h-2-2*pad)
	inner := w - 2
	var body []string
	for range pad {
		body = append(body, "")
	}
	switch {
	case !st.KeysLoaded:
		body = append(body, "  "+ui.Paint("Loading keys", ui.P().Muted))
	case len(st.Keys) == 0:
		body = append(body, "  "+ui.Paint("No keys yet", ui.P().Muted))
	default:
		d := m.delegate
		d.width, d.now = inner, now
		d.typeAt = 0
		if w >= 46 {
			d.typeAt = 3 + min(24, (w-8)*42/100)
		}
		d.showAgo = w >= 30
		m.list.SetSize(inner, max(1, n))
		rows := strings.Split(m.list.View(), "\n")
		for i := 0; i < n && i < len(rows); i++ {
			body = append(body, rows[i])
		}
	}
	right := ""
	if st.KeysLoaded {
		right = keyCount(st)
	}
	return strings.Split(ui.Panel(w, h, "Recent keys", right, true, body), "\n")
}

type delegate struct {
	width, typeAt int
	showAgo       bool
	now           time.Time
}

func (d *delegate) Height() int                         { return 1 }
func (d *delegate) Spacing() int                        { return 0 }
func (d *delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *delegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
	k := it.(item)
	sel := index == m.Index()
	name := ui.Sanitize(k.Name)
	ago, agoW := "", 0
	if d.showAgo {
		ago = core.Ago(k.LastUsedAt, d.now)
		agoW = ui.Width(ago)
	}
	nameW := d.width - 6
	if agoW > 0 {
		nameW -= agoW + 2
	}
	if d.typeAt > 0 {
		nameW = d.typeAt - 5
	}
	name = ui.Trunc(name, nameW)
	nameStyled := ui.Paint(name, ui.P().Text)
	if sel {
		nameStyled = ui.Bold(name, ui.P().Text)
	}
	c := "  " + nameStyled
	if d.typeAt > 0 {
		c = ui.Pad(c, d.typeAt-1) + ui.Paint(ui.Trunc(ui.Sanitize(k.Type), 8), ui.P().Muted)
	}
	if agoW > 0 {
		agoColor := ui.P().Muted
		if ago == "never" {
			agoColor = ui.P().Faint
		}
		c = ui.Pad(c, d.width-1-2-agoW) + ui.Paint(ago, agoColor)
	}
	if sel {
		fmt.Fprint(w, ui.SelLine(d.width, c))
		return
	}
	fmt.Fprint(w, " "+c)
}
