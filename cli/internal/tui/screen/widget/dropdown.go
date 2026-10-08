package widget

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type Dropdown struct {
	Items  []string
	Tail   []string
	Cur    int
	Anchor int
	MaxW   int
	Choose func(i int) tea.Msg

	cursor int
}

func (d *Dropdown) total() int { return len(d.Items) + len(d.Tail) }

func (d *Dropdown) extra() int {
	if len(d.Tail) == 0 {
		return 0
	}
	if len(d.Items) == 0 {
		return len(d.Tail)
	}
	return len(d.Tail) + 1
}

func (d *Dropdown) visible(st *core.State) int {
	_, bh := core.BodySize(st)
	room := max(bh-(d.Anchor+1), d.Anchor)
	k := len(d.Items)
	for k+d.extra()+2 > room && k > 1 {
		k--
	}
	return k
}

func (d *Dropdown) place(st *core.State) (x, y, w, h int) {
	bw, bh := core.BodySize(st)
	f := FrameFor(st, bw)
	w, h = min(d.MaxW, f.W), d.extra()+2
	if len(d.Items) > 0 {
		h += d.visible(st)
	}
	x = max(0, f.R-w)
	y = d.Anchor + 1
	if y+h > bh {
		y = max(0, d.Anchor-h)
	}
	return
}

func (d *Dropdown) Open(st *core.State) tea.Cmd {
	d.cursor = max(0, d.Cur)
	x, y, w, h := d.place(st)
	return core.Send(core.OpenOverlayMsg{Screen: d, X: x, Y: y, W: w, H: h})
}

func (d *Dropdown) Size(st *core.State, maxW, maxH int) (int, int) {
	_, _, w, h := d.place(st)
	return min(w, maxW), min(h, maxH)
}

func (d *Dropdown) Actions(*core.State) []core.Action {
	return []core.Action{{Key: ui.G.UpDown, Label: "Choose"}, {Key: "enter", Label: "Select"}, {Key: "esc", Label: "Close"}}
}

func (d *Dropdown) Update(msg tea.Msg, _ *core.State) (core.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	switch strings.ToLower(k.String()) {
	case "esc":
		return d, core.Close(d)
	case "up", "k":
		d.cursor = max(0, d.cursor-1)
	case "down", "j":
		d.cursor = min(d.total()-1, d.cursor+1)
	case "enter":
		if d.cursor == d.Cur {
			return d, core.Close(d)
		}
		return d, tea.Sequence(core.Close(d), core.Send(d.Choose(d.cursor)))
	}
	return d, nil
}

func (d *Dropdown) label(i int) string {
	if i < len(d.Items) {
		return ui.Sanitize(d.Items[i])
	}
	return ui.Sanitize(d.Tail[i-len(d.Items)])
}

func (d *Dropdown) View(_ *core.State, w, h int) string {
	p := ui.P()
	inner := w - 2
	item := func(i int) string {
		label := ui.Trunc(d.label(i), max(1, inner-6))
		mark := ""
		if i == d.Cur {
			mark = ui.Paint(ui.G.Check, p.Good)
		}
		gap := ui.Repeat(" ", max(0, inner-6-ui.Width(mark)-ui.Width(label)))
		if i == d.cursor {
			return ui.SelLine(inner, "  "+ui.Bold(label, p.Text)+gap+mark)
		}
		return "   " + ui.Paint(label, p.Text) + gap + mark
	}
	k := 0
	if len(d.Items) > 0 {
		k = max(0, h-2-d.extra())
	}
	start := 0
	if k > 0 {
		start = max(0, min(min(d.cursor, len(d.Items)-1)-k+1, len(d.Items)-k))
	}
	var body []string
	for i := start; i < start+k; i++ {
		body = append(body, item(i))
	}
	if len(d.Items) > 0 && len(d.Tail) > 0 {
		body = append(body, "   "+ui.Paint(ui.Repeat(ui.G.H, max(0, inner-6)), p.Rule))
	}
	for i := range d.Tail {
		body = append(body, item(len(d.Items)+i))
	}
	return ui.Panel(w, h, "", "", true, body)
}
