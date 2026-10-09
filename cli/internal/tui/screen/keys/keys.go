package keys

import (
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type item actions.KeySummary

func (i item) FilterValue() string { return i.Name }

type noDelegate struct{}

func (noDelegate) Height() int                                  { return 1 }
func (noDelegate) Spacing() int                                 { return 0 }
func (noDelegate) Update(tea.Msg, *list.Model) tea.Cmd          { return nil }
func (noDelegate) Render(io.Writer, list.Model, int, list.Item) {}

type Model struct {
	list     list.Model
	input    textinput.Model
	top      int
	want     string
	started  bool
	errs     map[string]string
	inflight map[string]bool
	signID   core.ID
	signName string
}

func New() *Model {
	l := list.New(nil, noDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "Search keys"
	in.CharLimit = 128
	return &Model{list: l, input: in, errs: map[string]string{}, inflight: map[string]bool{}}
}

func (m *Model) Capturing() bool { return m.input.Focused() }

func (m *Model) Select(name string) {
	m.input.Reset()
	m.input.Blur()
	m.want = name
}

func (m *Model) selected() (actions.KeySummary, bool) {
	it, ok := m.list.SelectedItem().(item)
	return actions.KeySummary(it), ok
}

func (m *Model) sync(st *core.State) {
	matches := actions.ResolveKeyQuery(st.Keys, ui.Sanitize(m.input.Value())).Matches
	cur := m.list.Items()
	same := len(cur) == len(matches)
	for i := 0; same && i < len(matches); i++ {
		same = cur[i].(item) == item(matches[i])
	}
	if same && m.want == "" {
		return
	}
	name := m.want
	if k, ok := m.selected(); name == "" && ok {
		name = k.Name
	}
	items := make([]list.Item, len(matches))
	for i, k := range matches {
		items[i] = item(k)
	}
	m.list.SetItems(items)
	m.list.SetSize(1, max(1, len(items)))
	i := slices.IndexFunc(matches, func(k actions.KeySummary) bool { return k.Name == name })
	m.list.Select(max(0, i))
	if i >= 0 || st.KeysLoaded {
		m.want = ""
	}
}

func narrow(st *core.State) bool { return st.Width < 80 }

func (m *Model) Actions(*core.State) []core.Action {
	out := []core.Action{
		{Key: "c", Label: "Copy public", Icon: ui.G.Icon.Copy},
		{Key: "p", Label: "Copy private", Icon: ui.G.Icon.Secret},
		{Key: "n", Label: "New key", Icon: ui.G.Icon.New},
		{Key: "/", Label: "Search", Icon: ui.G.Icon.Search},
		{Key: "f", Label: "Copy fingerprint", Icon: ui.G.Icon.Print},
		{Key: "g", Label: "Use for Git signing", Icon: ui.G.Icon.Sign},
		{Key: "r", Label: "Rename", Icon: ui.G.Icon.Rename},
		{Key: "d", Label: "Delete", Icon: ui.G.Icon.Delete, Danger: true},
		{Key: "i", Label: "Import keys", Icon: ui.G.Icon.Import},
		{Key: "e", Label: "Export vault", Icon: ui.G.Icon.Export},
	}
	return append([]core.Action{{Key: "enter", Label: "Details", Icon: ui.G.Icon.Details}}, out...)
}

func (m *Model) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	m.sync(st)
	var cmds []tea.Cmd
	if !m.started {
		m.started = true
		cmds = append(cmds, prefetch(st, m.inflight))
	}
	switch msg := msg.(type) {
	case core.KeysMsg:
		cmds = append(cmds, prefetch(st, m.inflight))
	case core.DetailMsg:
		m.onDetail(msg, st)
	case copyDoneMsg:
		if msg.err != nil {
			cmds = append(cmds, core.Toast(st.Reporter.Report("keys", msg.action, msg.err), ui.ToneBad))
		} else {
			cmds = append(cmds, core.Toast(msg.label+" copied", ui.ToneGood), core.Send(core.CancelClipMsg{Since: msg.since}))
		}
	case nameDoneMsg:
		if msg.err == nil {
			cmds = append(cmds, core.Send(core.KeysChangedMsg{}))
		}
	case deleteDoneMsg:
		if msg.gone || msg.deleted {
			cmds = append(cmds, core.Send(core.KeysChangedMsg{}))
		}
	case importDoneMsg:
		if msg.err == nil {
			cmds = append(cmds, core.Send(core.KeysChangedMsg{}))
			if len(msg.res.Failures) > 0 {
				cmds = append(cmds, logImportFailures(st, msg.res))
			}
		}
	case core.SigningMsg:
		if msg.ID != m.signID {
			break
		}
		if msg.Err != nil {
			cmds = append(cmds, core.Toast(st.Reporter.Report("keys", "use for git signing", msg.Err), ui.ToneBad))
		} else {
			cmds = append(cmds, core.Toast("Signing with "+m.signName, ui.ToneGood))
		}
	case tea.KeyPressMsg:
		cmds = append(cmds, m.key(msg, st))
	default:
		if m.input.Focused() {
			cmds = append(cmds, m.edit(msg, st))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) onDetail(msg core.DetailMsg, st *core.State) {
	delete(m.inflight, msg.Name)
	if msg.Err != nil {
		m.errs[msg.Name] = st.Reporter.Report("keys", "load key details", msg.Err)
	} else {
		delete(m.errs, msg.Name)
	}
}

func (m *Model) retry(st *core.State, name string) tea.Cmd {
	m.inflight[name] = true
	return loadDetail(st.Deps.ViewKey, name)
}

func (m *Model) edit(msg tea.Msg, st *core.State) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.sync(st)
	return cmd
}

func (m *Model) key(msg tea.KeyPressMsg, st *core.State) tea.Cmd {
	k := strings.ToLower(msg.String())
	if m.input.Focused() {
		switch k {
		case "esc":
			m.input.Reset()
			m.input.Blur()
			m.sync(st)
			return nil
		case "enter":
			m.input.Blur()
			return nil
		case "up":
			m.list.CursorUp()
			return nil
		case "down":
			m.list.CursorDown()
			return nil
		}
		return m.edit(msg, st)
	}
	switch k {
	case "up", "k":
		m.list.CursorUp()
	case "down", "j":
		m.list.CursorDown()
	case "/":
		return m.input.Focus()
	case "n":
		return widget.OpenModal(st, NewKeyModal())
	case "i":
		return widget.OpenModal(st, ImportModal())
	case "e":
		return widget.OpenModal(st, ExportModal())
	}
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	if k == "enter" {
		return widget.OpenModal(st, &detailsModal{m: m, name: sel.Name})
	}
	return m.act(k, sel, st)
}

func (m *Model) act(k string, sel actions.KeySummary, st *core.State) tea.Cmd {
	switch k {
	case "c":
		return core.CopyPublic(st, sel.Name, func(since core.ID, err error) tea.Msg {
			return copyDoneMsg{label: "Public key", action: "copy public key", since: since, err: err}
		})
	case "f":
		return copyFingerprint(st, sel.Fingerprint)
	case "p":
		if st.Deps.TerminalClipboard {
			return core.Toast("Private key copy needs a local session. Use Export instead", ui.ToneWarn)
		}
		return widget.OpenModal(st, PrivateModal(sel.Name))
	case "r":
		return widget.OpenModal(st, RenameModal(sel.Name))
	case "d":
		return widget.OpenModal(st, DeleteModal(sel))
	case "g":
		m.signID, m.signName = core.NextID(), ui.Sanitize(sel.Name)
		return enableSigning(st, m.signID, sel.Name)
	}
	return nil
}

func (m *Model) View(st *core.State, w, h int) string {
	m.sync(st)
	now := time.Now()
	lw := w
	if !narrow(st) {
		lw = 30
		if st.Width >= 100 {
			lw = 36
		}
	}
	left := strings.Split(m.listPane(st, lw, h), "\n")
	lines := left
	if !narrow(st) {
		rw := w - lw - 2
		var right []string
		if k, ok := m.selected(); ok {
			right = strings.Split(inspector(st, k, m.errs[k.Name], false, rw, h, false, now), "\n")
		} else {
			right = strings.Split(ui.Panel(rw, h, "Key", "", false, []string{"", "  " + ui.Paint("No key selected", ui.P().Muted)}), "\n")
		}
		lines = make([]string, h)
		for i := range lines {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			lines[i] = ui.Pad(l, lw) + "  " + r
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

func (m *Model) styleInput() {
	p := ui.P()
	s := m.input.Styles()
	for _, ss := range []*textinput.StyleState{&s.Focused, &s.Blurred} {
		ss.Text = ui.Fg(p.Text)
		ss.Placeholder = ui.Fg(p.Faint)
		ss.Prompt = ui.Fg(p.Muted)
	}
	m.input.SetStyles(s)
}

func center(w int, s string) string { return ui.Repeat(" ", (w-ui.Width(s))/2) + s }

func (m *Model) listPane(st *core.State, w, h int) string {
	p := ui.P()
	iw := w - 2
	items := m.list.Items()
	var body []string
	if h >= 10 {
		body = append(body, "")
	}
	m.styleInput()
	m.input.SetWidth(max(1, w-8))
	search := ui.Paint("/", p.Muted) + " " + ui.Paint("Search keys", p.Faint)
	if m.input.Focused() || m.input.Value() != "" {
		search = ui.Paint("/", p.Muted) + " " + m.input.View()
	}
	body = append(body, "  "+search, ui.Paint(ui.Repeat(ui.G.H, iw), p.Rule))
	n := max(0, h-2-len(body))
	idx := m.list.Index()
	if n > 0 {
		m.top = max(0, min(m.top, len(items)-n))
		if idx < m.top {
			m.top = idx
		} else if idx >= m.top+n {
			m.top = idx - n + 1
		}
	}
	switch {
	case len(items) == 0 && n > 0:
		body = append(body, emptyState(st, iw, n, m.input.Value() != "")...)
	default:
		showType := st.Width >= 100
		for i := m.top; i < len(items) && i < m.top+n; i++ {
			k := actions.KeySummary(items[i].(item))
			body = append(body, row(iw, k, i == idx, showType, st.KeySigns(k.Fingerprint)))
		}
	}
	right := ""
	if st.KeysLoaded {
		right = strconv.Itoa(len(items))
	}
	panel := strings.Split(ui.Panel(w, h, "Keys", right, true, body), "\n")
	if n > 0 && m.top+n < len(items) && w >= 4 {
		panel[len(panel)-1] = ui.Paint(ui.G.BL+ui.Repeat(ui.G.H, w-3), p.LineFocus) + ui.Paint(ui.G.Down, p.Faint) + ui.Paint(ui.G.BR, p.LineFocus)
	}
	return ui.Join(panel)
}

func row(iw int, k actions.KeySummary, sel, showType, signs bool) string {
	p := ui.P()
	kind, kw := "", 0
	if showType {
		kind = ui.Trunc(ui.Sanitize(k.Type), 10)
		kw = ui.Width(kind)
	}
	nameW := iw - 3
	if showType {
		nameW = iw - 3 - kw - 2
	}
	mark := ""
	if signs && ui.G.Icon.Sign != "" {
		mark = " " + ui.Paint(ui.G.Icon.Sign, p.Muted)
	}
	name := ui.Trunc(ui.Sanitize(k.Name), nameW-ui.Width(mark))
	styled := ui.Paint(name, p.Text)
	if sel {
		styled = ui.Bold(name, p.Text)
	}
	content := " " + styled + mark
	if showType {
		content = ui.Pad(content, iw-2-kw) + ui.Paint(kind, p.Muted) + " "
	}
	if sel {
		return ui.SelLine(iw, content)
	}
	return " " + content
}

func emptyState(st *core.State, iw, n int, searching bool) []string {
	p := ui.P()
	var block []string
	switch {
	case !st.KeysLoaded:
		block = []string{ui.Paint("Loading keys", p.Muted)}
	case searching:
		block = []string{ui.Paint("No keys match", p.Text), ui.Paint("Try a different search term", p.Muted)}
	default:
		block = []string{ui.Paint("No keys yet", p.Muted), "", ui.Button("New key", ui.Primary), "", ui.Button("Import keys", ui.Secondary)}
	}
	out := make([]string, 0, n)
	for range max(0, (n-len(block))/2) {
		out = append(out, "")
	}
	for _, l := range block {
		out = append(out, center(iw, ui.Trunc(l, iw)))
	}
	return out
}
