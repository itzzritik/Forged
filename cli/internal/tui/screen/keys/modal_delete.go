package keys

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type deleteModal struct {
	k       actions.KeySummary
	cancel  bool
	busy    string
	errMsg  string
	resolve bool
	id      core.ID
}

type deleteDoneMsg struct {
	id      core.ID
	k       actions.KeySummary
	gone    bool
	deleted bool
	err     error
}

func DeleteModal(k actions.KeySummary) widget.Modal { return &deleteModal{k: k} }

func (d *deleteModal) Actions(*core.State) []core.Action {
	if d.busy != "" {
		return nil
	}
	label := "Delete key"
	switch {
	case d.cancel:
		label = "Cancel"
	case d.needsResolve():
		label = "Retry"
	}
	return []core.Action{{Key: "enter", Label: label, Danger: label == "Delete key"}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Cancel"}}
}

func (d *deleteModal) needsResolve() bool { return d.resolve || d.k.Fingerprint == "" }

func (d *deleteModal) Spinning() bool { return d.busy != "" }

func (d *deleteModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case deleteDoneMsg:
		if msg.id != d.id || d.busy == "" {
			return d, nil
		}
		d.busy = ""
		switch {
		case msg.gone:
			return d, core.Close(d)
		case msg.err != nil:
			d.errMsg = st.Reporter.Report("keys", "delete key", msg.err)
			d.resolve = true
		case !msg.deleted:
			d.k, d.resolve, d.errMsg = msg.k, false, ""
		default:
			return d, tea.Sequence(core.Close(d), core.Toast("Key deleted", ui.ToneGood))
		}
	case tea.KeyPressMsg:
		if d.busy != "" {
			return d, nil
		}
		switch msg.String() {
		case "esc":
			return d, core.Close(d)
		case "tab", "shift+tab", "left", "right":
			d.cancel = !d.cancel
		case "enter":
			if d.cancel {
				return d, core.Close(d)
			}
			d.id, d.errMsg = core.NextID(), ""
			if d.needsResolve() {
				d.busy = "Checking key"
				return d, d.relist(st)
			}
			d.busy = "Deleting key"
			return d, d.remove(st)
		}
	}
	return d, nil
}

func (d *deleteModal) remove(st *core.State) tea.Cmd {
	del, id, k := st.Deps.DeleteKey, d.id, d.k
	return func() tea.Msg {
		if _, err := del(k.Name, k.Fingerprint); err != nil {
			return deleteDoneMsg{id: id, err: fmt.Errorf("deleting key: %w", err)}
		}
		return deleteDoneMsg{id: id, k: k, deleted: true}
	}
}

func (d *deleteModal) relist(st *core.State) tea.Cmd {
	list, id, name := st.Deps.ListKeys, d.id, d.k.Name
	return func() tea.Msg {
		all, err := list()
		if err != nil {
			return deleteDoneMsg{id: id, err: fmt.Errorf("reading keys: %w", err)}
		}
		i := slices.IndexFunc(all, func(k actions.KeySummary) bool { return k.Name == name })
		if i < 0 {
			return deleteDoneMsg{id: id, gone: true}
		}
		return deleteDoneMsg{id: id, k: all[i]}
	}
}

func (d *deleteModal) body(inner, limit, frame int) []string {
	p := ui.P()
	text := func(s string) string { return ui.Paint(s, p.Text) }
	muted := func(s string) string { return ui.Paint(s, p.Muted) }
	wrap := func(s string, paint func(string) string) []string {
		ls := ui.Wrap(s, inner)
		for i, l := range ls {
			ls[i] = paint(l)
		}
		return ls
	}
	l1 := wrap(ui.Sanitize(d.k.Name)+" will be removed from your vault.", text)
	l2 := wrap("Machines that sync this vault lose it too.", muted)
	if d.busy != "" || d.errMsg != "" {
		l2 = statusN(inner, len(l2), frame, d.busy, d.errMsg)
	}
	kind, fp := ui.Sanitize(d.k.Type), ui.Sanitize(d.k.Fingerprint)
	var kv []string
	if inner >= 30 {
		kv = []string{
			muted(ui.Pad("Type", 13)) + text(ui.Trunc(kind, inner-13)),
			muted(ui.Pad("Fingerprint", 13)) + text(ui.MidTrunc(fp, inner-13)),
		}
	} else {
		kv = []string{muted("Type"), text(ui.Trunc(kind, inner)), muted("Fingerprint"), text(ui.MidTrunc(fp, inner))}
	}
	del, can := ui.Button("Delete key", ui.DangerButton), ui.Button("Cancel", ui.Secondary)
	if d.cancel {
		del, can = ui.Button("Delete key", ui.Secondary), ui.Button("Cancel", ui.Primary)
	}
	btns := widget.Buttons(inner, del, can)
	build := func(pad int, rows, kv []string) []string {
		out := append(make([]string, pad), rows...)
		if len(kv) > 0 {
			out = append(out, "")
			out = append(out, kv...)
		}
		out = append(out, "")
		return append(out, btns...)
	}
	all := append(slices.Clone(l1), l2...)
	out := build(1, all, kv)
	if len(out) > limit {
		out = build(0, all, kv)
	}
	if len(out) > limit && (d.busy != "" || d.errMsg != "") {
		out = build(0, all, nil)
	}
	if len(out) > limit {
		out = build(0, l1, kv)
	}
	if len(out) > limit {
		keep := max(0, limit-len(btns))
		out = append(out[:min(keep, len(out)-len(btns))], btns...)
	}
	return out[:min(len(out), max(0, limit))]
}

func (d *deleteModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Delete key", "", d.body)
}

func (d *deleteModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, d.body)
}
