package health

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	twoColWidth = 120
	listWidth   = 46
)

type copyDoneMsg struct {
	id  core.ID
	err error
}

type Model struct {
	paths    config.Paths
	sel      int
	moved    bool
	copying  bool
	copyID   core.ID
	spinning bool
	checked  time.Time
}

func New(paths config.Paths) core.Screen { return &Model{paths: paths} }

func (m *Model) Spinning() bool { return m.spinning }

func (m *Model) selected(checks []Check) int {
	if m.moved {
		return max(0, min(m.sel, len(checks)-1))
	}
	for i, c := range checks {
		if c.Tone != ui.ToneGood {
			return i
		}
	}
	return 0
}

func (m *Model) busy(checks []Check) bool {
	if m.copying {
		return true
	}
	for _, c := range checks {
		if c.Tone == ui.ToneBusy {
			return true
		}
	}
	return false
}

func (m *Model) Actions(st *core.State) []core.Action {
	if m.copying {
		return []core.Action{{Key: ui.G.Spinner[st.SpinFrame%len(ui.G.Spinner)], Label: "Copying report"}}
	}
	var out []core.Action
	if st.CanFix() {
		out = append(out, core.Action{Key: "f", Label: "Fix issues", Icon: ui.G.Icon.Health})
	}
	out = append(out, core.Action{Key: "c", Label: "Copy report", Icon: ui.G.Icon.Copy})
	if !st.Recovery() {
		out = append(out, core.Action{Key: "r", Label: "Refresh", Icon: ui.G.Icon.Refresh})
	}
	return append(out, core.Action{Key: ui.G.UpDown, Label: "Select"})
}

func (m *Model) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case core.SnapshotMsg:
		if msg.Err == nil {
			m.checked = time.Now()
		}
	case core.LockedMsg:
		if m.copying {
			m.copying, st.Busy.Clipboard = false, false
		}
	case copyDoneMsg:
		if msg.id != m.copyID || !m.copying {
			break
		}
		m.copying = false
		st.Busy.Clipboard = false
		if msg.err != nil {
			text := st.Reporter.Report("health", "copy report", msg.err)
			cmd = core.Toast(text, ui.ToneBad)
			break
		}
		cmd = tea.Sequence(core.Toast("Report copied", ui.ToneGood), core.Send(core.CancelClipMsg{Since: msg.id}))
	case tea.KeyPressMsg:
		cmd = m.key(strings.ToLower(msg.String()), st)
	}
	m.spinning = m.busy(Checks(st, m.paths))
	return m, cmd
}

func (m *Model) key(k string, st *core.State) tea.Cmd {
	checks := Checks(st, m.paths)
	switch k {
	case "up", "k":
		m.move(checks, -1)
	case "down", "j":
		m.move(checks, 1)
	case "f":
		if st.CanFix() && !st.Busy.Clipboard {
			return core.Send(core.RequestRepairMsg{})
		}
	case "c":
		return m.copyReport(st)
	case "r":
		if st.Recovery() || st.Busy.Clipboard {
			return nil
		}
		return tea.Batch(core.Send(core.RefreshMsg{}), core.LoadSecurityCmd(st))
	}
	return nil
}

func (m *Model) move(checks []Check, d int) {
	m.sel = max(0, min(m.selected(checks)+d, len(checks)-1))
	m.moved = true
}

func (m *Model) copyReport(st *core.State) tea.Cmd {
	if st.Busy.Clipboard {
		return nil
	}
	st.Busy.Clipboard, m.copying, m.copyID = true, true, core.NextID()
	report, copyText, id := Report(st, m.paths), st.Deps.CopyText, m.copyID
	return func() tea.Msg {
		if err := copyText(report); err != nil {
			return copyDoneMsg{id, fmt.Errorf("copying report: %w", err)}
		}
		return copyDoneMsg{id: id}
	}
}

func (m *Model) View(st *core.State, w, h int) string {
	checks := Checks(st, m.paths)
	if w <= 0 || h <= 0 {
		return ""
	}
	bh := 1
	if st.Height >= 24 {
		bh = 3
	}
	lines := strings.Split(m.banner(st, w, bh), "\n")
	lines = append(lines, "")
	if ra := h - len(lines); ra > 0 && len(checks) > 0 {
		sel := m.selected(checks)
		if st.Width >= twoColWidth {
			lines = append(lines, m.twoCol(st, checks, sel, w, ra)...)
		} else {
			lines = append(lines, m.oneCol(st, checks, sel, w, ra)...)
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

func (m *Model) banner(st *core.State, w, bh int) string {
	n := Count(st, m.paths)
	fix, action := "", ""
	if st.CanFix() && !st.Busy.Clipboard {
		fix, action = "f", "Fix issues"
	}
	switch {
	case strings.TrimSpace(st.RepairErr) != "":
		return ui.Banner(w, bh, ui.ToneBad, "Fix issues failed: "+ui.Sanitize(strings.TrimSpace(st.RepairErr)), fix, action)
	case st.CanFix():
		text := "Forged found problems it can fix."
		if n > 0 {
			text = fmt.Sprintf("Forged found %s it can fix.", core.Plural(n, "problem"))
		}
		return ui.Banner(w, bh, ui.ToneWarn, text, fix, action)
	case n > 0:
		verb := "need"
		if n == 1 {
			verb = "needs"
		}
		return ui.Banner(w, bh, ui.ToneWarn, fmt.Sprintf("%s %s attention.", core.Plural(n, "problem"), verb), "", "")
	}
	return m.verdict(w, bh)
}

func (m *Model) verdict(w, bh int) string {
	text := "  " + ui.Bold(ui.G.Check, ui.P().Good) + " " + ui.Paint("Everything is working", ui.P().Text)
	if !m.checked.IsZero() {
		ago := "Checked " + core.Ago(m.checked.UTC().Format(time.RFC3339), time.Now())
		if gap := w - ui.Width(text) - ui.Width(ago) - 2; gap >= 2 {
			text += ui.Repeat(" ", gap) + ui.Paint(ago, ui.P().Muted)
		}
	}
	lines := make([]string, bh)
	lines[bh/2] = text
	return ui.Join(lines)
}

func glyph(st *core.State, c Check) string {
	g := ui.G.Check
	switch c.Tone {
	case ui.ToneWarn:
		g = ui.G.Warn
	case ui.ToneBad:
		g = ui.G.Cross
	case ui.ToneBusy:
		return ui.SpinnerGlyph(st.SpinFrame)
	}
	return ui.Paint(g, ui.ToneColor(c.Tone))
}

func (m *Model) rows(st *core.State, checks []Check, sel, listW, n int) []string {
	start := 0
	if sel >= n {
		start = sel - n + 1
	}
	sx := 20
	if listW >= 40 {
		sx = 24
	}
	out := make([]string, 0, n)
	for j, c := range checks[start : start+n] {
		on := start+j == sel
		name := ui.Paint(ui.Trunc(ui.Sanitize(c.Name), sx-7), ui.P().Text)
		if on {
			name = ui.Bold(ui.Trunc(ui.Sanitize(c.Name), sx-7), ui.P().Text)
		}
		statusColor := ui.P().Muted
		switch c.Tone {
		case ui.ToneBad:
			statusColor = ui.P().Danger
		case ui.ToneWarn:
			statusColor = ui.P().Warn
		}
		content := ui.Pad("  "+glyph(st, c)+" "+name, sx-1) + ui.Paint(ui.Trunc(c.Status, listW-sx-2), statusColor)
		if j == n-1 && start+n < len(checks) {
			content = ui.Pad(ui.Trunc(content, listW-3), listW-3) + ui.Paint(ui.G.Down, ui.P().Faint)
		}
		if on {
			out = append(out, ui.SelLine(listW, content))
		} else {
			out = append(out, " "+content)
		}
	}
	return out
}

func detailLines(c Check, w int) []string {
	return ui.Wrap(ui.Sanitize(c.Detail), w)
}

func (m *Model) twoCol(st *core.State, checks []Check, sel, w, ra int) []string {
	listW := min(listWidth, w)
	rows := m.rows(st, checks, sel, listW, min(ra, len(checks)))
	dw := w - listW - 6
	detail := detailLines(checks[sel], dw)
	rule := ui.Paint(ui.G.V, ui.P().Line)
	out := make([]string, ra)
	for i := range out {
		left := ui.Repeat(" ", listW)
		if i < len(rows) {
			left = ui.Pad(rows[i], listW)
		}
		right := ""
		switch {
		case i == 0:
			right = ui.Bold(ui.Trunc(ui.Sanitize(checks[sel].Name), dw), ui.P().Text)
		case i >= 2 && i-2 < len(detail):
			right = ui.Paint(detail[i-2], ui.P().Muted)
		}
		out[i] = left + "  " + rule + " " + right
	}
	return out
}

func (m *Model) oneCol(st *core.State, checks []Check, sel, w, ra int) []string {
	detail := detailLines(checks[sel], w-6)
	n := min(max(3, ra-(min(len(detail), 3)+3)), ra, len(checks))
	detail = detail[:min(len(detail), 3, max(0, ra-n-3))]
	if ra-n < 3 {
		return m.rows(st, checks, sel, w, min(ra, len(checks)))
	}
	out := m.rows(st, checks, sel, w, n)
	tail := []string{
		"",
		ui.Paint(ui.Repeat(ui.G.H, w), ui.P().Line),
		"   " + ui.Bold(ui.Trunc(ui.Sanitize(checks[sel].Name), w-6), ui.P().Text),
	}
	for _, l := range detail {
		tail = append(tail, "   "+ui.Paint(l, ui.P().Muted))
	}
	return append(out, tail[:min(len(tail), max(0, ra-n))]...)
}
