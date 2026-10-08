package keys

import (
	"fmt"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/widget"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

// privateCopy hands a late lease to exactly one owner: the modal that accepts it or whoever cancels first.
type privateCopy struct {
	mu       sync.Mutex
	canceled bool
	accepted bool
	lease    core.ClipboardLease
}

func (p *privateCopy) offer(l core.ClipboardLease) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.canceled {
		return false
	}
	p.lease = l
	return true
}

func (p *privateCopy) accept() (core.ClipboardLease, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.canceled || p.lease == nil {
		return nil, false
	}
	p.accepted = true
	return p.lease, true
}

func (p *privateCopy) cancel() core.ClipboardLease {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.canceled = true
	if p.accepted {
		return nil
	}
	l := p.lease
	p.lease = nil
	return l
}

type privateDoneMsg struct {
	tok *privateCopy
	err error
}

type privateModal struct {
	name    string
	sec     ui.Secret
	cur     *privateCopy
	errMsg  string
	pending bool
}

func PrivateModal(name string) widget.Modal {
	return &privateModal{name: name, sec: ui.NewSecret(128)}
}

func (p *privateModal) Capturing() bool { return true }

func (p *privateModal) Actions(*core.State) []core.Action {
	if p.pending {
		return []core.Action{{Key: "esc", Label: "Cancel"}}
	}
	return []core.Action{{Key: "enter", Label: "Copy"}, {Key: "esc", Label: "Cancel"}}
}

func (p *privateModal) abort(st *core.State) tea.Cmd {
	p.sec.Wipe()
	p.pending = false
	st.Busy.Clipboard = false
	if p.cur == nil {
		return nil
	}
	l := p.cur.cancel()
	p.cur = nil
	if l == nil {
		return nil
	}
	return staleClear(l)
}

func (p *privateModal) Update(msg tea.Msg, st *core.State) (core.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case core.LockedMsg:
		return p, p.abort(st)
	case privateDoneMsg:
		if msg.tok != p.cur || !p.pending {
			if l := msg.tok.cancel(); l != nil {
				return p, staleClear(l)
			}
			return p, nil
		}
		p.pending = false
		st.Busy.Clipboard = false
		if msg.err != nil {
			p.cur = nil
			p.errMsg = st.Reporter.Report("keys", "copy private key", msg.err)
			return p, nil
		}
		lease, ok := msg.tok.accept()
		p.cur = nil
		if !ok {
			return p, nil
		}
		clip := &core.Clip{ID: core.NextID(), Name: p.name, Lease: lease, Until: time.Now().Add(clipWindow)}
		return p, tea.Sequence(core.Close(p), core.Send(core.ClipStartMsg{Clip: clip}))
	case tea.PasteMsg:
		if !p.pending {
			p.errMsg = ""
			p.sec.Update(msg)
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return p, tea.Sequence(core.Close(p), p.abort(st))
		case "enter":
			if p.pending {
				return p, nil
			}
			if p.sec.Len() == 0 {
				p.errMsg = "Enter your master password"
				return p, nil
			}
			p.errMsg, p.pending, p.cur = "", true, &privateCopy{}
			st.Busy.Clipboard = true
			return p, p.run(st, p.cur, p.sec.Take())
		default:
			if !p.pending {
				p.errMsg = ""
				p.sec.Update(msg)
			}
		}
	}
	return p, nil
}

func (p *privateModal) run(st *core.State, tok *privateCopy, pw []byte) tea.Cmd {
	view, copySecret, name := st.Deps.ViewFullKey, st.Deps.CopySensitiveText, p.name
	return func() tea.Msg {
		defer clear(pw)
		d, err := view(name, pw)
		if err != nil {
			return privateDoneMsg{tok: tok, err: fmt.Errorf("decrypting private key: %w", err)}
		}
		lease, err := copySecret(d.PrivateKey)
		if err != nil {
			return privateDoneMsg{tok: tok, err: fmt.Errorf("copying private key: %w", err)}
		}
		if !tok.offer(lease) {
			return staleClear(lease)()
		}
		return privateDoneMsg{tok: tok}
	}
}

func (p *privateModal) Spinning() bool { return p.pending }

func (p *privateModal) body(inner, limit, frame int) []string {
	busy := ""
	if p.pending {
		busy = "Decrypting key"
	}
	lines := ui.Wrap("Enter your master password to copy this private key.", inner)
	for i, l := range lines {
		lines[i] = ui.Paint(l, ui.P().Text)
	}
	lines = append(lines, "", p.sec.View(inner, !p.pending))
	lines = append(lines, status(inner, frame, busy, p.errMsg)...)
	return pad(lines, limit)
}

func (p *privateModal) View(st *core.State, w, h int) string {
	return widget.ModalView(st, w, h, "Copy private key", "", p.body)
}

func (p *privateModal) Size(st *core.State, maxW, maxH int) (int, int) {
	return widget.ModalSize(st, 56, maxW, maxH, p.body)
}
