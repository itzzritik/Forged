package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/account"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	longWarn   = 60
	maxPending = 5
)

type notice struct {
	title, text string
	tone        ui.Tone
}

func (n notice) long() bool { return n.tone != ui.ToneGood && ui.Width(n.text) > longWarn }

func (n notice) heading() string {
	switch {
	case n.title != "":
		return n.title
	case strings.HasPrefix(n.text, "Signed out"):
		return "Signed out"
	case n.tone == ui.ToneBad:
		return "Error"
	}
	return "Warning"
}

type infoModal struct{ title, text string }

func (a *app) openInfo(n notice) { a.openUnder(&infoModal{title: n.heading(), text: n.text}) }

// openUnder goes under any open modal so it never interrupts one the user is still using.
func (a *app) openUnder(m widget.Modal) {
	w, h := m.Size(a.st, a.st.Width-4, a.st.Height-2)
	a.overlays = append([]core.OpenOverlayMsg{{Screen: m, W: w, H: h, Center: true, Dim: true}}, a.overlays...)
}

func (a *app) offerHeadlessUnlock() {
	acc, ok := a.screens[core.TabAccount].(*account.Model)
	if !ok || !a.life.offerHeadless || a.inGate || a.st.Locked {
		return
	}
	a.life.offerHeadless = false
	if m := acc.HeadlessOffer(a.st); m != nil {
		a.openUnder(m)
	}
}

func (a *app) notify(text string, tone ui.Tone) tea.Cmd {
	return a.post(notice{text: text, tone: tone})
}

func (a *app) warn(title, text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if a.inGate {
		if text != "" {
			a.gate.SetError(text)
		}
		return nil
	}
	return a.post(notice{title: title, text: text, tone: ui.ToneWarn})
}

// post never draws on the gate: notices wait there until the dashboard shows again.
func (a *app) post(n notice) tea.Cmd {
	n.text = strings.TrimSpace(n.text)
	switch {
	case n.text == "":
		return nil
	case !a.inGate && n.long():
		a.openInfo(n)
		return nil
	case a.inGate || a.toastQueued || len(a.pending) > 0:
		a.pending = append(a.pending, n)
		a.pending = a.pending[max(0, len(a.pending)-maxPending):]
		return nil
	}
	return a.showToast(n, false)
}

func (a *app) flush() tea.Cmd {
	short := a.pending[:0]
	for _, n := range a.pending {
		if n.long() {
			a.openInfo(n)
		} else {
			short = append(short, n)
		}
	}
	a.pending = short
	if t := a.toast; t != nil {
		return a.showToast(notice{text: t.Text, tone: t.Tone}, true)
	}
	return a.nextToast()
}

func (a *app) nextToast() tea.Cmd {
	if len(a.pending) == 0 {
		return nil
	}
	n := a.pending[0]
	a.pending = a.pending[1:]
	return a.showToast(n, true)
}

func (a *app) showToast(n notice, queued bool) tea.Cmd {
	a.toast, a.toastUntil, a.toastQueued = &core.ToastMsg{Text: n.text, Tone: n.tone}, time.Now().Add(toastFor), queued
	if a.toastOn {
		return nil
	}
	a.toastOn = true
	return tick(toastFor, toastMsg{})
}

func (a *app) expireToast() tea.Cmd {
	if left := time.Until(a.toastUntil); left > 0 {
		return tick(left, toastMsg{})
	}
	a.toastOn = false
	// A toast hidden by the gate stays until flush replays it.
	if a.inGate {
		return nil
	}
	a.toast, a.toastQueued = nil, false
	return a.nextToast()
}

func (m *infoModal) Actions(*core.State) []core.Action {
	return []core.Action{{Key: "enter", Label: "OK"}, {Key: "esc", Label: "Close"}}
}

func (m *infoModal) Update(msg tea.Msg, _ *core.State) (core.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "enter" || k.String() == "esc") {
		return m, core.Close(m)
	}
	return m, nil
}

func (m *infoModal) body(inner, limit, _ int) []string {
	lines := []string{""}
	for _, l := range ui.Wrap(ui.Sanitize(m.text), inner) {
		lines = append(lines, ui.Paint(l, ui.P().Text))
	}
	lines = append(append(lines, ""), widget.Buttons(inner, ui.Button("OK", ui.Primary), "")...)
	if limit < len(lines) {
		lines = append(lines[:max(0, limit-1)], lines[len(lines)-1])
	}
	return lines[:min(len(lines), max(0, limit))]
}

func (m *infoModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, m.body)
}

func (m *infoModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, m.title, "", m.body)
}
