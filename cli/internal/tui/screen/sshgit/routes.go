package sshgit

import (
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const pollEvery = 2 * time.Second

type pollMsg struct{ id core.ID }

func loadCmd(st *core.State, id core.ID) tea.Cmd {
	load := st.Deps.LoadSSHRoutingDebug
	return func() tea.Msg {
		d, err := load()
		if err != nil {
			err = fmt.Errorf("reading SSH routes: %w", err)
		}
		return core.RoutesMsg{ID: id, Debug: d, Err: err}
	}
}

func pollCmd(id core.ID) tea.Cmd {
	return tea.Tick(pollEvery, func(time.Time) tea.Msg { return pollMsg{id} })
}

func routeLabel(r actions.SSHRouteDebug) string {
	switch {
	case r.Kind == "git" && r.Owner != "" && r.Repo != "":
		return ui.Sanitize(strings.TrimPrefix(r.Host+"/", "/") + r.Owner + "/" + r.Repo)
	case r.User != "" && r.Host != "":
		return ui.Sanitize(fmt.Sprintf("%s@%s:%d", r.User, r.Host, r.Port))
	}
	return ui.Sanitize(r.Target)
}

func keyLabel(r actions.SSHRouteDebug) string {
	for _, v := range []string{r.KeyName, r.KeyRef} {
		if v = strings.TrimSpace(v); v != "" {
			return ui.Sanitize(v)
		}
	}
	if fp := strings.TrimSpace(r.Fingerprint); fp != "" {
		fp = strings.TrimPrefix(fp, "SHA256:")
		return ui.Sanitize("SHA256:" + fp[:min(12, len(fp))])
	}
	return "unknown key"
}

func serviceLabel(r actions.SSHRouteDebug) string {
	switch strings.ToLower(strings.TrimSpace(r.Host)) {
	case "github.com":
		return "GitHub"
	case "gitlab.com":
		return "GitLab"
	case "bitbucket.org":
		return "Bitbucket"
	}
	switch r.Kind {
	case "git":
		if h := strings.TrimSpace(r.Host); h != "" {
			return ui.Sanitize(h)
		}
		return "Git"
	case "ssh":
		return "SSH"
	}
	return "Route"
}

func lastUsed(r actions.SSHRouteDebug, now time.Time) string {
	t := r.Updated
	if r.LastSuccessAt != nil {
		t = *r.LastSuccessAt
	}
	return core.Ago(t.Format(time.RFC3339), now)
}

type routeRow struct{ target, key, service, last string }

func routeRows(routes []actions.SSHRouteDebug, now time.Time) []routeRow {
	out := make([]routeRow, len(routes))
	for i, r := range routes {
		out[i] = routeRow{routeLabel(r), keyLabel(r), serviceLabel(r), lastUsed(r, now)}
	}
	return out
}

type routesModal struct {
	m      *Model
	sel    int
	target string
	top    int
}

func (r *routesModal) Spinning() bool { return !r.m.loaded && r.m.loadErr == "" }

func (r *routesModal) Size(st *core.State, maxW, maxH int) (int, int) {
	w, h := core.BodySize(st)
	return min(w, maxW), min(h, maxH)
}

func (r *routesModal) cur() int {
	routes := r.m.debug.Routes
	if i := slices.IndexFunc(routes, func(x actions.SSHRouteDebug) bool { return x.Target == r.target }); i >= 0 && r.target != "" {
		return i
	}
	return max(0, min(r.sel, len(routes)-1))
}

func (r *routesModal) route() (actions.SSHRouteDebug, bool) {
	if len(r.m.debug.Routes) == 0 {
		return actions.SSHRouteDebug{}, false
	}
	return r.m.debug.Routes[r.cur()], true
}

func (r *routesModal) Actions(st *core.State) []core.Action {
	var out []core.Action
	if _, ok := r.route(); ok && !st.Recovery() {
		out = append(out, core.Action{Key: "x", Label: "Forget route", Icon: ui.G.Icon.Delete, Danger: true})
	}
	if r.m.canForgetAll(st) {
		out = append(out, core.Action{Key: "a", Label: "Forget all", Icon: ui.G.Icon.Delete, Danger: true})
	}
	return append(out, core.Action{Key: "esc", Label: "Close"})
}

func (r *routesModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if _, resized := msg.(tea.WindowSizeMsg); resized {
			r.scroll(st)
		}
		return r, nil
	}
	m := r.m
	switch strings.ToLower(k.String()) {
	case "esc":
		if m.routes == r {
			m.routes = nil
		}
		return r, core.Close(r)
	case "up", "k":
		r.move(-1)
		r.scroll(st)
	case "down", "j":
		r.move(1)
		r.scroll(st)
	case "x":
		if rt, ok := r.route(); ok && !st.Recovery() {
			return r, m.openConfirm(st, &confirmModal{target: rt.Target, label: routeLabel(rt)})
		}
	case "a":
		if m.canForgetAll(st) {
			return r, m.openConfirm(st, &confirmModal{all: true, guard: m.debug.RuntimeGuardRequired, count: len(m.debug.Routes)})
		}
	}
	return r, nil
}

func (r *routesModal) move(d int) {
	routes := r.m.debug.Routes
	r.sel = max(0, min(r.cur()+d, len(routes)-1))
	r.target = ""
	if r.sel < len(routes) {
		r.target = routes[r.sel].Target
	}
}

func (r *routesModal) View(st *core.State, w, h int) string {
	right := ""
	if r.m.loaded {
		right = core.Plural(len(r.m.debug.Routes), "route")
	}
	return ui.Panel(w, h, "Learned routes", right, true, r.body(st, w-2, h-2))
}

func window(top, sel, room, total int) int {
	if room <= 0 {
		return 0
	}
	top = max(min(top, sel), sel-room+1)
	return max(0, min(top, total-room))
}

func (r *routesModal) scroll(st *core.State) {
	w, h := core.BodySize(st)
	room := h - 2 - len(r.notes(st, w-2, h-2)) - 1
	r.top = window(r.top, r.cur(), room, len(r.m.debug.Routes))
}

func (r *routesModal) notes(st *core.State, w, h int) []string {
	p := ui.P()
	m := r.m
	line := func(s string, c color.Color) string { return build(0, cell{2, ui.Paint(ui.Trunc(s, w-3), c)}) }
	var out []string
	if h >= 10 {
		out = append(out, "")
	}
	if m.debug.RuntimeGuardRequired {
		out = append(out, line(ui.G.Warn+" SSH route guard needs a reset", p.Warn))
	}
	switch {
	case !m.loaded && m.loadErr == "":
		out = append(out, build(0, cell{2, ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Reading routes", p.Muted)}))
	case m.loadErr != "":
		out = append(out, line(m.loadErr, p.Danger))
	case len(m.debug.Routes) == 0:
		out = append(out, line("Nothing learned yet. Routes appear after your first connection.", p.Muted))
	}
	return out
}

func (r *routesModal) body(st *core.State, w, h int) []string {
	p := ui.P()
	m := r.m
	e := w - 1
	out := r.notes(st, w, h)
	if !m.loaded || len(m.debug.Routes) == 0 {
		return out
	}
	rows := routeRows(m.debug.Routes, time.Now())
	lastW, svcW := ui.Width("Last used"), ui.Width("Service")
	for _, rr := range rows {
		lastW, svcW = max(lastW, ui.Width(rr.last)), max(svcW, ui.Width(rr.service))
	}
	svcX, keyX := 0, 0
	if e >= 64 {
		svcX = e - lastW - 2 - svcW
	}
	if e >= 46 {
		keyX = e - lastW - 16
		if svcX > 0 {
			keyX = svcX - 16
		}
	}
	end := keyX
	if end == 0 {
		end = e - lastW
	}
	tgtW := max(1, end-4)
	out = append(out, build(0, cell{2, ui.Paint("Target", p.Faint)}, cell{keyX, ui.Paint(onlyIf(keyX > 0, "Key"), p.Faint)},
		cell{svcX, ui.Paint(onlyIf(svcX > 0, "Service"), p.Faint)}, at(e, ui.Paint("Last used", p.Faint))))
	room := h - len(out)
	sel := r.cur()
	top := window(r.top, sel, room, len(rows))
	for i := top; i < len(rows) && i-top < room; i++ {
		rr := rows[i]
		tgt := ui.Trunc(rr.target, tgtW)
		cs := []cell{{2, ui.Paint(tgt, p.Text)}, {keyX, ui.Paint(onlyIf(keyX > 0, ui.Trunc(rr.key, 14)), p.Muted)},
			{svcX, ui.Paint(onlyIf(svcX > 0, rr.service), p.Muted)}, at(e, ui.Paint(rr.last, p.Muted))}
		if i == sel {
			cs[0] = cell{2, ui.Bold(tgt, p.Text)}
			out = append(out, ui.SelLine(w, build(1, cs...)))
			continue
		}
		out = append(out, build(0, cs...))
	}
	return out
}

func onlyIf(ok bool, s string) string {
	if ok {
		return s
	}
	return ""
}
