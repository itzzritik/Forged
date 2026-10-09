package tui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

func (a *app) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	return v
}

func (a *app) render() string {
	w, h := a.st.Width, a.st.Height
	if a.veil {
		return ""
	}
	if a.floor() {
		return ui.Floor(w, h)
	}
	var base string
	var layers []ui.Layer
	if a.inGate {
		base = a.gate.View(a.st, w, h)
	} else {
		base = a.dashboard(w, h)
		layers = a.overlayLayers(w, h)
	}
	if a.toast != nil && !a.inGate {
		layers = append(layers, a.toastLayer(w))
	}
	if len(layers) == 0 {
		return base
	}
	return ui.Compose(w, h, base, layers...)
}

func (a *app) dashboard(w, h int) string {
	st := a.st
	titles := make([]string, len(a.tabs))
	for i, t := range a.tabs {
		titles[i] = tabTitles[t]
	}
	chip := st.Chip()
	if chip.Tone == ui.ToneBusy {
		chip.Glyph = ui.SpinnerGlyph(st.SpinFrame)
	}
	lines := make([]string, core.HeaderY(st))
	lines = append(lines, strings.Split(ui.Header(w, titles, max(0, slices.Index(a.tabs, a.active)), chip), "\n")...)
	bw, bh := core.BodySize(st)
	_, oy := core.BodyOrigin(st)
	for len(lines) < oy {
		lines = append(lines, "")
	}
	body := strings.Split(a.screens[a.active].View(st, bw, bh), "\n")
	for i := range bh {
		l := ""
		if i < len(body) {
			l = ui.Trunc(body[i], bw)
		}
		lines = append(lines, "  "+l)
	}
	for len(lines) < h-1 {
		lines = append(lines, "")
	}
	return ui.Join(append(lines[:h-1], a.footer(w)))
}

func (a *app) footer(w int) string {
	acts := a.top().Actions(a.st)
	left := make([]ui.Hint, 0, 4)
	for _, x := range acts[:min(4, len(acts))] {
		left = append(left, ui.Hint{Key: x.Key, Label: x.Label})
	}
	_, open := a.top().(*menu)
	dash := len(a.overlays) == 0
	var right []ui.Hint
	if !open && !a.capturing() && (dash || len(acts) > len(left) || ui.HintsWidth(left, true) > w-4) {
		right = append(right, ui.Hint{Key: "m", Label: "More"})
	}
	if dash && !a.capturing() {
		right = append(right, ui.Hint{Key: "q", Label: "Quit"})
	}
	return ui.Footer(w, left, right)
}

func (a *app) overlayLayers(w, h int) []ui.Layer {
	st := a.st
	ox, oy := core.BodyOrigin(st)
	bw, bh := core.BodySize(st)
	out := make([]ui.Layer, 0, len(a.overlays))
	for _, o := range a.overlays {
		ow, oh := o.W, o.H
		if s, ok := o.Screen.(core.Sizer); ok {
			ow, oh = s.Size(st, w-4, h-2)
		}
		ow, oh = max(1, min(ow, w-4)), max(1, min(oh, h-2))
		l := ui.Layer{Fill: ui.P().Surface, Dim: o.Center || o.Dim}
		if o.Center {
			l.X, l.Y = (w-ow)/2, max(1, (h-oh)/2)
		} else {
			l.X = max(min(ox+o.X, ox+bw-ow), min(ox, w-ow))
			l.Y = max(min(oy+o.Y, oy+bh-oh), min(oy, h-oh))
		}
		l.Content = o.Screen.View(st, ow, oh)
		out = append(out, l)
	}
	return out
}

func (a *app) toastLayer(w int) ui.Layer {
	_, oy := core.BodyOrigin(a.st)
	_, bh := core.BodySize(a.st)
	t := ui.Toast(ui.Trunc(ui.Sanitize(a.toast.Text), w-10), a.toast.Tone)
	return ui.Layer{Content: t, X: max(0, w-2-ui.Width(t)), Y: oy + bh - 1}
}

type menu struct {
	items  []ui.MenuItem
	keys   []string
	cursor int
	owner  core.Screen
}

type menuRunMsg struct {
	owner core.Screen
	key   string
}

func (m *menu) add(x core.Action) {
	shown := x.Key
	if ui.Width(shown) > 2 {
		shown = " "
	}
	m.items = append(m.items, ui.MenuItem{Key: shown, Icon: x.Icon, Label: x.Label, Danger: x.Danger})
	m.keys = append(m.keys, x.Key)
}

func newMenu(owner core.Screen, acts []core.Action, global bool) *menu {
	m := &menu{owner: owner}
	seen := map[string]bool{"l": global, "q": global}
	for _, x := range acts {
		if seen[x.Key] || core.Press(x.Key).Code == 0 {
			continue
		}
		seen[x.Key] = true
		m.add(x)
	}
	if !global {
		return m
	}
	if len(m.items) > 0 {
		m.items = append(m.items, ui.MenuItem{Sep: true})
		m.keys = append(m.keys, "")
	}
	m.add(core.Action{Key: "l", Label: "Lock", Icon: ui.G.Icon.Lock})
	m.add(core.Action{Key: "q", Label: "Quit", Icon: ui.G.Icon.Quit})
	return m
}

func (m *menu) Actions(*core.State) []core.Action {
	return []core.Action{{Key: ui.G.UpDown, Label: "Move"}, {Key: "enter", Label: "Run"}, {Key: "esc", Label: "Close"}}
}

func (m *menu) run(key string) tea.Cmd {
	return tea.Sequence(core.Close(m), core.Send(menuRunMsg{owner: m.owner, key: key}))
}

func (m *menu) Update(msg tea.Msg, _ *core.State) (core.Screen, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	k := strings.ToLower(press.String())
	switch k {
	case "enter":
		return m, m.run(m.keys[m.cursor])
	case "esc":
		return m, core.Close(m)
	}
	if k != "" && slices.Contains(m.keys, k) {
		return m, m.run(k)
	}
	switch k {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	}
	return m, nil
}

func (m *menu) move(d int) {
	for i := m.cursor + d; i >= 0 && i < len(m.items); i += d {
		if !m.items[i].Sep {
			m.cursor = i
			return
		}
	}
}

func (m *menu) View(st *core.State, w, h int) string {
	return ui.Menu(w, "Actions", m.items, m.cursor, h)
}

func (m *menu) Size(st *core.State, maxW, maxH int) (int, int) {
	_, bh := core.BodySize(st)
	w := min(34, maxW)
	return w, strings.Count(m.View(st, w, min(maxH, bh)), "\n") + 1
}
