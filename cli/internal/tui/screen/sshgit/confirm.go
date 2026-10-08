package sshgit

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type forgetDoneMsg struct {
	id  core.ID
	err error
}

type confirmClosedMsg struct{}

type confirmModal struct {
	target, label string
	all, guard    bool
	count         int
	cancel        bool
	busy          bool
	errMsg        string
	id            core.ID
}

func (c *confirmModal) title() string {
	if c.all {
		return "Forget all routes?"
	}
	return "Forget this route?"
}

func (c *confirmModal) confirmLabel() string {
	if c.all {
		return "Forget all routes"
	}
	return "Forget route"
}

func (c *confirmModal) Spinning() bool { return c.busy }

func (c *confirmModal) Actions(*core.State) []core.Action {
	if c.busy {
		return nil
	}
	label := c.confirmLabel()
	if c.cancel {
		label = "Cancel"
	}
	return []core.Action{{Key: "enter", Label: label, Danger: !c.cancel}, {Key: "tab", Label: "Switch"}, {Key: "esc", Label: "Cancel"}}
}

func (c *confirmModal) dismiss() tea.Cmd {
	return tea.Sequence(core.Close(c), core.Send(confirmClosedMsg{}))
}

func (c *confirmModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case forgetDoneMsg:
		if msg.id != c.id || !c.busy {
			return c, nil
		}
		c.busy = false
		if msg.err != nil {
			c.errMsg = st.Reporter.Report("ssh", "forget routes", msg.err)
			return c, nil
		}
		text := "Route forgotten"
		if c.all {
			text = "All routes forgotten"
		}
		return c, tea.Sequence(core.Close(c), core.Toast(text, ui.ToneGood), core.Send(confirmClosedMsg{}))
	case tea.KeyPressMsg:
		if c.busy {
			return c, nil
		}
		switch msg.String() {
		case "esc":
			return c, c.dismiss()
		case "tab", "shift+tab", "left", "right":
			c.cancel = !c.cancel
		case "enter":
			if c.cancel {
				return c, c.dismiss()
			}
			c.id, c.busy, c.errMsg = core.NextID(), true, ""
			return c, c.run(st)
		}
	}
	return c, nil
}

func (c *confirmModal) run(st *core.State) tea.Cmd {
	one, all, id, target, isAll := st.Deps.ClearSSHRoute, st.Deps.ClearAllSSHRoutes, c.id, c.target, c.all
	return func() tea.Msg {
		var err error
		if isAll {
			err = all()
		} else {
			err = one(target)
		}
		if err != nil {
			err = fmt.Errorf("forgetting routes: %w", err)
		}
		return forgetDoneMsg{id: id, err: err}
	}
}

func (c *confirmModal) body(inner, limit, frame int) []string {
	p := ui.P()
	var head, sub string
	switch {
	case c.all && c.guard:
		head, sub = "Reset all learned SSH routes.", "Close active SSH sessions first."
		if c.count > 0 {
			head = fmt.Sprintf("%d learned routes will be forgotten.", c.count)
		}
	case c.all:
		head, sub = fmt.Sprintf("%d learned routes will be forgotten.", c.count), "Forged asks again the next time you connect."
		if c.count == 1 {
			head = "1 learned route will be forgotten."
		}
	default:
		head, sub = c.label+" will be forgotten.", "Forged asks again the next time you connect."
	}
	var lines []string
	for _, l := range ui.Wrap(head, inner) {
		lines = append(lines, ui.Paint(l, p.Text))
	}
	switch {
	case c.busy:
		lines = append(lines, ui.SpinnerGlyph(frame)+" "+ui.Paint("Forgetting", p.Text))
	case c.errMsg != "":
		for _, l := range ui.Wrap(c.errMsg, inner) {
			lines = append(lines, ui.Paint(l, p.Danger))
		}
	default:
		for _, l := range ui.Wrap(sub, inner) {
			lines = append(lines, ui.Paint(l, p.Muted))
		}
	}
	del, can := ui.Button(c.confirmLabel(), ui.DangerButton), ui.Button("Cancel", ui.Secondary)
	if c.cancel {
		del, can = ui.Button(c.confirmLabel(), ui.Secondary), ui.Button("Cancel", ui.Primary)
	}
	btns := widget.Buttons(inner, del, can)
	out := append([]string{""}, lines...)
	out = append(append(out, ""), btns...)
	if len(out) > limit {
		out = append(lines, btns...)
	}
	if len(out) > limit {
		keep := max(0, limit-len(btns))
		out = append(out[:min(keep, len(lines))], btns...)
	}
	return out[:min(len(out), max(0, limit))]
}

func (c *confirmModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, c.body)
}

func (c *confirmModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, c.title(), "", c.body)
}
