package keys

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const clipWindow = 45 * time.Second

type metaRow struct {
	label, value string
	good         bool
}

func clipFor(st *core.State, name string) bool { return st.Clip != nil && st.Clip.Name == name }

func inspector(st *core.State, k actions.KeySummary, errText string, retry bool, w, h int, focused bool, now time.Time) string {
	cd := clipFor(st, k.Name)
	title := ui.Sanitize(k.Name)
	return ui.Panel(w, h, title, "", focused, inspectorBody(st, k, errText, retry, w-6, h-2, cd, now))
}

func inspectorBody(st *core.State, k actions.KeySummary, errText string, retry bool, cw, rows int, cd bool, now time.Time) []string {
	p := ui.P()
	lines := make([]string, max(0, rows))
	lab := 12
	if cw >= 46 {
		lab = 14
	}
	end := len(lines)
	if cd {
		end -= 4
	}
	r := 1
	put := func(s string) {
		if r < end {
			lines[r] = "  " + s
			r++
		}
	}
	kv := func(label, value string) {
		put(ui.Paint(ui.Pad(ui.Trunc(label, lab-1), lab), p.Muted) + ui.Paint(ui.Trunc(value, cw-lab), p.Text))
	}
	d, loaded := st.Details[k.Name]
	fp := k.Fingerprint
	if loaded && d.Fingerprint != "" {
		fp = d.Fingerprint
	}
	kind := k.Type
	if loaded && d.Type != "" {
		kind = d.Type
	}

	pub := ui.Sanitize(d.PublicKey)
	var metas []metaRow
	if loaded {
		signing := "Off"
		if st.KeySigns(d.Fingerprint) || d.GitSigning {
			signing = "On"
		}
		used := d.LastUsedAt
		if used == "" {
			used = k.LastUsedAt
		}
		metas = []metaRow{
			{"Git signing", signing, signing == "On"},
			{"Last used", core.Ago(used, now), false},
			{"Created", orDash(core.Date(d.CreatedAt)), false},
			{"Device", orDash(ui.Sanitize(d.DeviceOrigin)), false},
			{"Comment", orDash(ui.Sanitize(firstOf(d.Comment, k.Comment))), false},
		}
	}
	n := 1
	if loaded {
		n = len(ui.CodeBlock(cw, pub, false))
	}
	mode, keep := "block", len(metas)
	need := func() int {
		switch mode {
		case "block":
			return 2 + n + 4 + 1 + keep
		case "tight":
			return 2 + n + 2 + 1 + keep
		}
		return 2 + 3 + 1 + keep
	}
	degrade := func() bool {
		switch {
		case keep > 3:
			keep--
		case mode == "block":
			mode = "tight"
		case mode == "tight":
			mode = "line"
		case keep > 0:
			keep--
		default:
			return false
		}
		return true
	}
	for loaded && need() > end-r && degrade() {
	}
	kv("Type", ui.Sanitize(kind))
	kv("Fingerprint", ui.MidTrunc(ui.Sanitize(fp), cw-lab))
	r++
	switch {
	case errText != "":
		for _, l := range ui.Wrap(errText, cw) {
			put(ui.Paint(l, p.Danger))
		}
		if retry {
			put(ui.Paint("Press enter to retry", p.Muted))
		}
	case !loaded:
		put(ui.Paint("Reading key details", p.Muted))
	default:
		put(ui.Paint("Public key", p.Muted))
		block := ui.CodeBlock(cw, pub, true)
		tight := ui.CodeBlock(cw, pub, false)
		switch {
		case mode == "block" && r+len(block) <= end:
			for _, l := range block {
				put(l)
			}
		case mode != "line" && r+len(tight) <= end:
			for _, l := range tight {
				put(l)
			}
		case r < end:
			put(ui.CodeBlock(cw, ui.MidTrunc(pub, cw-2), false)[0])
		}
		r++
		for _, m := range metas[:keep] {
			if m.good {
				put(ui.Paint(ui.Pad(m.label, lab), p.Muted) + ui.Paint(ui.G.Dot, p.Good) + " " + ui.Paint(m.value, p.Text))
				continue
			}
			kv(m.label, m.value)
		}
	}
	if cd && rows >= 10 {
		remaining := time.Until(st.Clip.Until)
		secs := max(0, int(math.Ceil(remaining.Seconds())))
		lines[rows-3] = "  " + ui.Paint(ui.G.Check, p.Good) + " " + ui.Paint(ui.Trunc(fmt.Sprintf("Private key copied. Clears in %ds", secs), cw-2), p.Muted)
		lines[rows-2] = "  " + ui.Progress(cw, remaining.Seconds()/clipWindow.Seconds())
	}
	return lines
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type detailsModal struct {
	m       *Model
	name    string
	renamed core.ID
}

func (d *detailsModal) key(st *core.State) (actions.KeySummary, bool) {
	i := slices.IndexFunc(st.Keys, func(k actions.KeySummary) bool { return k.Name == d.name })
	if i < 0 {
		return actions.KeySummary{}, false
	}
	return st.Keys[i], true
}

func (d *detailsModal) Actions(st *core.State) []core.Action {
	var out []core.Action
	for _, a := range d.m.Actions(st) {
		if slices.Contains([]string{"c", "p", "f", "g", "r", "d"}, a.Key) {
			out = append(out, a)
		}
	}
	return append(out, core.Action{Key: "esc", Label: "Close"})
}

func (d *detailsModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case nameDoneMsg:
		if msg.err == nil && msg.old == d.name {
			d.name, d.renamed = msg.name, core.NextID()
		}
	case deleteDoneMsg:
		if msg.deleted && msg.k.Name == d.name {
			return d, core.Close(d)
		}
	case core.KeysMsg:
		// A list requested before the rename still has the old name.
		if _, ok := d.key(st); !ok && msg.Err == nil && msg.ID > d.renamed && st.KeysLoaded {
			return d, core.Close(d)
		}
	case tea.KeyPressMsg:
		k := strings.ToLower(msg.String())
		switch {
		case k == "enter" && d.m.errs[d.name] != "":
			return d, d.m.retry(st, d.name)
		case k == "enter", k == "esc":
			return d, core.Close(d)
		}
		if sel, ok := d.key(st); ok {
			return d, d.m.act(k, sel, st)
		}
	}
	return d, nil
}

func (d *detailsModal) View(st *core.State, w, h int) string {
	if k, ok := d.key(st); ok {
		return inspector(st, k, d.m.errs[k.Name], true, w, h, true, time.Now())
	}
	return ui.Panel(w, h, ui.Sanitize(d.name), "", true, nil)
}

func modalHeight(st *core.State, k actions.KeySummary, w, limit int) int {
	rows := 9
	if d, ok := st.Details[k.Name]; ok {
		rows = len(ui.CodeBlock(w-6, ui.Sanitize(d.PublicKey), false)) + 14
	}
	if clipFor(st, k.Name) {
		rows += 4
	}
	return min(limit, rows+2)
}

func (d *detailsModal) Size(st *core.State, maxW, maxH int) (int, int) {
	w := max(1, min(64, maxW-4))
	_, bh := core.BodySize(st)
	limit := max(3, bh)
	if k, ok := d.key(st); ok {
		return w, modalHeight(st, k, w, limit)
	}
	return w, min(limit, 11)
}
