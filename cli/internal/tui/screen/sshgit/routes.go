package sshgit

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
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

func (m *Model) routesBlock(st *core.State, f widget.Frame, selIdx, room int) []string {
	p := ui.P()
	now := time.Now()
	title := ui.Paint("Learned routes", p.Muted)
	cells := []cell{{0, title}}
	if m.loaded && f.W >= 40 {
		cells = append(cells, at(f.W, ui.Paint(core.Plural(len(m.debug.Routes), "route"), p.Faint)))
	}
	out := []string{f.Pad(build(0, cells...))}
	room--
	if m.debug.RuntimeGuardRequired && room > 0 {
		out = append(out, f.Pad(build(0, cell{2, ui.Paint(ui.G.Warn+" SSH route guard needs a reset", p.Warn)})))
		room--
	}
	switch {
	case !m.loaded && m.loadErr != "":
		return append(out, f.Pad(build(0, cell{2, ui.Paint(ui.Trunc(m.loadErr, f.W-2), p.Danger)})))
	case !m.loaded:
		return append(out, f.Pad(build(0, cell{2, ui.SpinnerGlyph(st.SpinFrame) + ui.Paint(" Reading routes", p.Muted)})))
	case len(m.debug.Routes) == 0:
		return append(out, f.Pad(build(0, cell{2, ui.Paint(ui.Trunc("Nothing learned yet. Routes appear after your first connection.", f.W-2), p.Muted)})))
	}
	rows := routeRows(m.debug.Routes, now)
	lastW, svcW := ui.Width("Last used"), ui.Width("Service")
	for _, r := range rows {
		lastW, svcW = max(lastW, ui.Width(r.last)), max(svcW, ui.Width(r.service))
	}
	svcX, keyX := 0, 0
	if f.W >= 64 {
		svcX = f.W - lastW - 2 - svcW
	}
	if f.W >= 46 {
		keyX = f.W - lastW - 16
		if svcX > 0 {
			keyX = svcX - 16
		}
	}
	end := keyX
	if end == 0 {
		end = f.W - lastW
	}
	tgtW := max(1, end-4)
	if m.loadErr != "" && room > 0 {
		out = append(out, f.Pad(build(0, cell{2, ui.Paint(ui.Trunc(m.loadErr, f.W-2), p.Danger)})))
		room--
	}
	if room > 0 {
		out = append(out, f.Pad(build(0, cell{2, ui.Paint("Target", p.Faint)}, cell{keyX, ui.Paint(onlyIf(keyX > 0, "Key"), p.Faint)},
			cell{svcX, ui.Paint(onlyIf(svcX > 0, "Service"), p.Faint)}, at(f.W, ui.Paint("Last used", p.Faint)))))
		room--
	}
	sel := selIdx - firstRoute
	top := max(0, min(m.top, len(rows)-max(room, 0)))
	for i := top; i < len(rows) && i-top < room; i++ {
		r := rows[i]
		on := i == sel
		tgt := ui.Trunc(r.target, tgtW)
		cs := []cell{{2, ui.Paint(tgt, p.Text)}, {keyX, ui.Paint(onlyIf(keyX > 0, ui.Trunc(r.key, 14)), p.Muted)},
			{svcX, ui.Paint(onlyIf(svcX > 0, r.service), p.Muted)}, at(f.W, ui.Paint(r.last, p.Muted))}
		if on {
			cs[0] = cell{1, ui.Bold(tgt, p.Text)}
			out = append(out, f.Pad(ui.SelLine(f.W, build(1, cs...))))
			continue
		}
		out = append(out, f.Pad(build(0, cs...)))
	}
	return out
}

func onlyIf(ok bool, s string) string {
	if ok {
		return s
	}
	return ""
}
