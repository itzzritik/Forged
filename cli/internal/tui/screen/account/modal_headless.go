package account

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type headlessDoneMsg struct {
	id                  core.ID
	enable, skip, later bool
	err                 error
}

type headlessModal struct {
	owner  *Model
	turnOn bool
	busy   bool
	id     core.ID
}

func headlessOffered(st *core.State) bool {
	s := st.Security
	return st.SecurityLoaded && st.SecurityErr == "" && s.HeadlessSupported && !s.HeadlessUnlock && !s.HeadlessOffered &&
		(st.Deps.Remote || s.SystemAuthCapability != "available")
}

func (m *Model) HeadlessOffer(st *core.State) widget.Modal {
	if !headlessOffered(st) {
		return nil
	}
	return &headlessModal{owner: m}
}

func (h *headlessModal) Spinning() bool { return h.busy }

func (h *headlessModal) Actions(*core.State) []core.Action {
	if h.busy {
		return nil
	}
	label := "Not now"
	if h.turnOn {
		label = "Turn on"
	}
	return []core.Action{{Key: "enter", Label: label}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Not now"}}
}

func (h *headlessModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case headlessDoneMsg:
		if msg.id == h.id && h.busy {
			h.busy = false
			return h, core.Close(h)
		}
	case tea.KeyPressMsg:
		if h.busy {
			return h, nil
		}
		switch msg.String() {
		case "tab", "shift+tab", "left", "right":
			h.turnOn = !h.turnOn
		case "esc":
			return h, h.skip(st)
		case "enter":
			if !h.turnOn {
				return h, h.skip(st)
			}
			h.busy, h.id = true, core.NextID()
			return h, h.owner.setHeadless(st, true, h.id)
		}
	}
	return h, nil
}

func (h *headlessModal) skip(st *core.State) tea.Cmd {
	if st.Security.HeadlessOffered {
		return core.Close(h)
	}
	st.Security.HeadlessOffered = true
	save := st.Deps.SkipHeadlessOffer
	return tea.Batch(core.Close(h), func() tea.Msg {
		err := save()
		if err != nil {
			err = fmt.Errorf("saving automatic unlock answer: %w", err)
		}
		return headlessDoneMsg{skip: true, err: err}
	})
}

func (h *headlessModal) body(inner, limit, frame int) []string {
	p := ui.P()
	var lines []string
	for _, s := range ui.Wrap("SSH and Git signing keep working after restarts and logouts, without your master password. Turn this on only for a server you trust.", inner) {
		lines = append(lines, ui.Paint(s, p.Muted))
	}
	if h.busy {
		lines = []string{ui.SpinnerGlyph(frame) + " " + ui.Paint("Turning on", p.Text)}
	}
	on, later := ui.Button("Turn on", ui.Secondary), ui.Button("Not now", ui.Primary)
	if h.turnOn {
		on, later = ui.Button("Turn on", ui.Primary), ui.Button("Not now", ui.Secondary)
	}
	btns := widget.Buttons(inner, on, later)
	all := append(append(append([]string{""}, lines...), ""), btns...)
	if len(all) > limit {
		all = append(lines, btns...)
	}
	if len(all) > limit {
		all = append(all[:min(max(0, limit-len(btns)), len(lines))], btns...)
	}
	return all[:min(len(all), max(0, limit))]
}

func (h *headlessModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, h.body)
}

func (h *headlessModal) View(st *core.State, w, ht int) string {
	return widget.ModalView(st, w, ht, "Keep Forged unlocked on this server?", "", h.body)
}
