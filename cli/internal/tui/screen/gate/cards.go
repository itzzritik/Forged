package gate

import (
	"runtime"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type lines struct {
	rows    []string
	compact bool
	spin    string
}

func (l *lines) add(s ...string) { l.rows = append(l.rows, s...) }
func (l *lines) gap()            { l.rows = append(l.rows, "") }

func (l *lines) soft() {
	if !l.compact {
		l.gap()
	}
}

func authLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "Touch ID"
	case "windows":
		return "Windows Hello"
	}
	return "System Auth"
}

func (m *Model) footer(l *lines, W int, items []ui.Hint, label string) string {
	if m.busy != "" {
		return l.spin + " " + ui.Paint(ui.Trunc(ui.Sanitize(m.busy), W-2), ui.P().Text)
	}
	btn := ui.Button(label, ui.Primary)
	bw := ui.Width(btn)
	room := W - bw - 2
	var last []ui.Hint
	if len(items) > 0 {
		last = items[len(items)-1:]
	}
	left, lw := "", 0
	for _, c := range []struct {
		h   []ui.Hint
		lab bool
	}{{items, true}, {last, true}, {items, false}, {last, false}} {
		if hw := ui.HintsWidth(c.h, c.lab); hw <= room {
			left, lw = ui.Hints(c.h, c.lab), hw
			break
		}
	}
	return left + ui.Repeat(" ", W-lw-bw) + btn
}

func (m *Model) errRows(W int) []string {
	if m.err == "" {
		return nil
	}
	var out []string
	for _, l := range ui.Wrap(ui.Sanitize(m.err), W) {
		out = append(out, ui.Paint(l, ui.P().Danger))
	}
	return out
}

func (m *Model) fieldErr(i, W int) []string {
	if m.errAt != i {
		return nil
	}
	return m.errRows(W)
}

func title(s string) string { return ui.Bold(s, ui.P().Text) }
func muted(s string) string { return ui.Paint(s, ui.P().Muted) }
func label(s string) string { return muted(s) }

func (m *Model) rows(st *core.State, cw int, compact bool) []string {
	pad := 4
	if cw < 40 {
		pad = 2
	}
	W := cw - 2*pad
	l := &lines{compact: compact, spin: ui.SpinnerGlyph(st.SpinFrame)}
	switch m.kind {
	case Launch, Busy:
		l.gap()
		l.soft()
		if m.err != "" {
			l.add(m.errRows(W)...)
		} else {
			text := "Checking this machine"
			if m.busy != "" {
				text = ui.Sanitize(m.busy)
			}
			line := l.spin + " " + ui.Paint(ui.Trunc(text, W-2), ui.P().Text)
			l.add(ui.Repeat(" ", (W-ui.Width(line))/2) + line)
		}
		l.soft()
		l.gap()
	case Welcome:
		m.welcome(l, W)
	case RepairAccount:
		l.soft()
		l.add(title("Repair account"))
		l.soft()
		wrapped := ui.Wrap(ui.Sanitize(m.err), W)
		if len(wrapped) > 3 {
			wrapped = wrapped[:3]
			wrapped[2] = ui.Trunc(wrapped[2]+ui.G.Ellipsis, W)
		}
		for i := 0; i < 3 && (i < len(wrapped) || !compact); i++ {
			if i < len(wrapped) {
				l.add(muted(wrapped[i]))
			} else {
				l.gap()
			}
		}
		l.gap()
		l.add(m.footer(l, W, []ui.Hint{{Key: "esc", Label: "Quit"}}, "Repair account"))
		l.soft()
	case Create:
		m.create(l, W)
	case Login:
		m.login(l, W)
	case Restore:
		l.soft()
		l.add(title("Restore your vault"))
		if !compact {
			l.add(muted(ui.Trunc(ui.Sanitize("Logged in as "+st.AccountEmail), W)))
		}
		m.password(l, W)
		l.add(m.footer(l, W, []ui.Hint{{Key: "esc", Label: "Back"}}, "Restore"))
		l.soft()
	case Unlock:
		l.soft()
		l.add(title("Unlock Forged"))
		if email := ui.Sanitize(st.AccountEmail); email != "" && !compact {
			l.add(muted(ui.Trunc(email, W)))
		}
		if m.prompt != "" {
			for _, s := range ui.Wrap(ui.Sanitize(m.prompt), W) {
				l.add(muted(s))
			}
		}
		var hints []ui.Hint
		switch {
		case m.waiting:
			hints = []ui.Hint{{Key: "esc", Label: "Use password"}}
		case m.hasTrust:
			hints = []ui.Hint{{Key: "tab", Label: authLabel()}, {Key: "esc", Label: "Quit"}}
		default:
			hints = []ui.Hint{{Key: "esc", Label: "Quit"}}
		}
		m.password(l, W)
		l.add(m.footer(l, W, hints, "Unlock"))
		l.soft()
	}
	ind := ui.Repeat(" ", pad-1)
	for i, r := range l.rows {
		if r != "" {
			l.rows[i] = ind + r
		}
	}
	return l.rows
}

func (m *Model) password(l *lines, W int) {
	l.gap()
	if m.waiting {
		l.add(label(authLabel()), l.spin+" "+ui.Paint(ui.Trunc("Waiting for confirmation", W-2), ui.P().Text))
	} else {
		l.add(label("Master password"), m.a.View(W, true))
	}
	l.add(m.errRows(W)...)
	l.gap()
}

func (m *Model) create(l *lines, W int) {
	l.soft()
	l.add(title("Create a local vault"))
	l.add(label("Master password"), m.a.View(W, m.focus == 0))
	l.add(m.fieldErr(0, W)...)
	l.add(label("Confirm master password"), m.b.View(W, m.focus == 1))
	l.add(m.fieldErr(1, W)...)
	l.gap()
	if !l.compact {
		for _, s := range ui.Wrap("If you lose this password, your keys can't be recovered.", W) {
			l.add(ui.Paint(s, ui.P().Warn))
		}
	}
	l.add(m.footer(l, W, []ui.Hint{{Key: "tab", Label: "Next"}, {Key: "esc", Label: "Back"}}, "Create vault"))
	l.soft()
}

func (m *Model) welcome(l *lines, W int) {
	type opt struct{ icon, title, sub string }
	opts := []opt{{ui.G.Icon.Login, "Log in to Forged", "Sync encrypted keys across machines"}, {ui.G.Icon.New, "Create a local vault", "Keys stay on this machine only"}}
	l.soft()
	l.add(title("Welcome to Forged"))
	l.soft()
	for i, o := range opts {
		iw := ui.IconWidth(o.icon)
		t, s := ui.Trunc(o.title, W-3-iw), ui.Repeat(" ", iw)+muted(ui.Trunc(o.sub, W-3-iw))
		if i == m.cursor {
			l.add(ui.SelLine(W, " "+ui.Icon(o.icon, ui.P().Accent)+ui.Bold(t, ui.P().Text)), ui.SelLine(W, " "+s))
		} else {
			l.add("  "+ui.Icon(o.icon, ui.P().Muted)+ui.Paint(t, ui.P().Text), "  "+s)
		}
		if i == 0 && !l.compact {
			l.gap()
		}
	}
	l.add(m.errRows(W)...)
	l.gap()
	l.add(m.footer(l, W, []ui.Hint{{Key: ui.G.UpDown, Label: "Choose"}, {Key: "esc", Label: "Quit"}}, "Continue"))
	l.soft()
}

func (m *Model) login(l *lines, W int) {
	l.soft()
	l.add(title("Log in to Forged"))
	if !l.compact {
		l.add(muted(ui.Trunc("Approve this code in your browser.", W)))
		l.gap()
	}
	code := ui.Sanitize(m.code)
	if spaced := strings.Join(strings.Split(code, ""), " "); W >= ui.Width(spaced)+2 {
		code = spaced
	}
	code = ui.Trunc(code, W)
	left := (W - ui.Width(code)) / 2
	strip := ui.Fg(ui.P().Text).Background(ui.P().Sel).Bold(true).Render(ui.Repeat(" ", left) + code + ui.Repeat(" ", W-left-ui.Width(code)))
	l.add(strip)
	if !l.compact {
		l.gap()
	}
	status := m.status
	if status == "" {
		status = "Waiting for approval"
	}
	if m.committing {
		status = "Saving account securely"
	}
	l.add(l.spin + " " + muted(ui.Trunc(ui.Sanitize(status), W-2)))
	if m.url != "" {
		for _, s := range strings.Split(ansi.Hardwrap(ui.Sanitize(m.url), W, true), "\n") {
			l.add(ui.Paint(s, ui.P().Steel))
		}
	}
	l.add(m.errRows(W)...)
	l.gap()
	var hints []ui.Hint
	if !m.committing {
		hints = []ui.Hint{{Key: "c", Label: "Copy link"}, {Key: "esc", Label: "Cancel"}}
	}
	l.add(m.footer(l, W, hints, "Open link"))
	l.soft()
}
