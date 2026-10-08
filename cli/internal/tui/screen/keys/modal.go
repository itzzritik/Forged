package keys

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

func pad(lines []string, limit int) []string {
	limit = max(0, limit)
	switch {
	case len(lines)+2 <= limit:
		lines = append([]string{""}, append(lines, "")...)
	case len(lines)+1 <= limit:
		lines = append([]string{""}, lines...)
	}
	return lines[:min(len(lines), limit)]
}

func status(inner, frame int, busy, errText string) []string {
	return statusN(inner, 2, frame, busy, errText)
}

func statusN(inner, n, frame int, busy, errText string) []string {
	p := ui.P()
	out := make([]string, n)
	switch {
	case busy != "":
		out[0] = ui.SpinnerGlyph(frame) + " " + ui.Paint(ui.Trunc(busy, inner-2), p.Text)
	case errText != "":
		ls := ui.Wrap(errText, inner)
		if len(ls) > n {
			ls[n-1] = ui.Trunc(ls[n-1]+ui.G.Ellipsis, inner)
			ls = ls[:n]
		}
		for i, l := range ls {
			out[i] = ui.Paint(l, p.Danger)
		}
	}
	return out
}

func staleClear(lease core.ClipboardLease) tea.Cmd {
	return func() tea.Msg {
		for try := range 10 {
			if _, err := lease.ClearIfUnchanged(); err == nil {
				return nil
			}
			if try < 3 {
				time.Sleep(5 * time.Second)
			} else {
				time.Sleep(30 * time.Second)
			}
		}
		return nil
	}
}

func notice(inner, n, frame int, busy, errText, warn string) []string {
	if busy == "" && errText == "" && warn != "" {
		out := make([]string, n)
		for i, l := range ui.Wrap(warn, inner) {
			if i < n {
				out[i] = ui.Paint(l, ui.P().Warn)
			}
		}
		return out
	}
	return statusN(inner, n, frame, busy, errText)
}

func clipLines(lines []string, n, inner int) []string {
	if n < 0 {
		n = 0
	}
	if len(lines) <= n {
		return lines
	}
	lines = lines[:n]
	if n > 0 {
		lines[n-1] = ui.Trunc(lines[n-1]+ui.G.Ellipsis, inner)
	}
	return lines
}

func windowStart(cur, n, total int) int {
	return max(0, min(cur-n+1, total-n))
}
