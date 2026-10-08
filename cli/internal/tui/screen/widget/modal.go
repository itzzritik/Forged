package widget

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

// Flush prefixes a body line that already carries its own indent.
const Flush = "\x00"

type Modal interface {
	core.Screen
	core.Sizer
}

func OpenModal(st *core.State, m Modal) tea.Cmd {
	w, h := m.Size(st, st.Width-4, st.Height-2)
	return core.Send(core.OpenOverlayMsg{Screen: m, W: w, H: h, Center: true, Dim: true})
}

func ModalSize(st *core.State, capW, maxW, maxH int, body func(inner, limit, frame int) []string) (int, int) {
	w := max(1, min(capW, maxW))
	limit := max(3, maxH)
	return w, min(len(body(max(1, w-6), limit-2, st.SpinFrame))+2, limit)
}

func ModalView(st *core.State, w, h int, title, right string, body func(inner, limit, frame int) []string) string {
	lines := body(max(1, w-6), h-2, st.SpinFrame)
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, Flush):
			lines[i] = l[len(Flush):]
		case l != "":
			lines[i] = "  " + l
		}
	}
	return ui.Panel(w, h, title, right, true, lines)
}

func Buttons(inner int, primary, secondary string) []string {
	pw, sw := ui.Width(primary), ui.Width(secondary)
	right := func(s string, w int) string { return ui.Repeat(" ", inner-w) + s }
	switch {
	case secondary == "":
		return []string{right(primary, pw)}
	case pw+2+sw > inner:
		return []string{right(primary, pw), "", right(secondary, sw)}
	}
	return []string{right(secondary+"  "+primary, sw+2+pw)}
}
