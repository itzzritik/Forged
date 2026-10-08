package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/flame"
	"github.com/itzzritik/forged/cli/internal/tui/screen/account"
	"github.com/itzzritik/forged/cli/internal/tui/screen/gate"
	"github.com/itzzritik/forged/cli/internal/tui/screen/health"
	"github.com/itzzritik/forged/cli/internal/tui/screen/keys"
	"github.com/itzzritik/forged/cli/internal/tui/screen/overview"
	"github.com/itzzritik/forged/cli/internal/tui/screen/sshgit"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	frameEvery = 50 * time.Millisecond
	spinEvery  = time.Second / 12
	toastFor   = 3 * time.Second
)

var (
	allTabs   = []core.Tab{core.TabOverview, core.TabKeys, core.TabSSH, core.TabAccount, core.TabHealth}
	tabTitles = [...]string{"Overview", "Keys", "SSH & Git", "Account", "Health"}
)

type frameMsg struct{ gen int }

type spinMsg struct{ gen int }

type toastMsg struct{}

type app struct {
	st       *core.State
	intent   Intent
	paths    config.Paths
	ctx      context.Context
	cancel   context.CancelFunc
	gate     *gate.Model
	inGate   bool
	animate  bool
	tabs     []core.Tab
	screens  map[core.Tab]core.Screen
	active   core.Tab
	overlays []core.OpenOverlayMsg

	toast       *core.ToastMsg
	toastUntil  time.Time
	toastOn     bool
	toastQueued bool
	pending     []notice
	frameGen    int
	frameOn     bool
	spinGen     int
	spinOn      bool

	life          life
	pendingSelect string
}

func newApp(intent Intent, deps Deps, paths config.Paths) *app {
	st := &core.State{Deps: deps, Reporter: core.NewReporter(deps), Details: map[string]actions.KeyDetail{}}
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{
		st: st, intent: intent, paths: paths, ctx: ctx, cancel: cancel,
		gate: gate.New(), inGate: true, animate: flame.Animate(),
		tabs: st.Tabs(), active: core.TabOverview,
		screens: map[core.Tab]core.Screen{
			core.TabOverview: overview.New(),
			core.TabKeys:     keys.New(),
			core.TabSSH:      sshgit.New(),
			core.TabAccount:  account.New(),
			core.TabHealth:   health.New(paths),
		},
	}
	if intent.Doctor {
		a.active = core.TabHealth
	}
	return a
}

func (a *app) Init() tea.Cmd {
	// sshgit polls routes until it first learns it is hidden.
	return tea.Batch(tea.RequestBackgroundColor, a.broadcast(core.SwitchTabMsg{Tab: a.active}), a.startBoot())
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.update(msg)
	return a, tea.Batch(cmd, a.timers())
}

func (a *app) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg:
		return tea.Batch(a.input(msg), a.touchIdle())
	case tea.BackgroundColorMsg:
		ui.SetBackground(msg.Color, msg.IsDark())
		return nil
	case tea.WindowSizeMsg:
		a.st.Width, a.st.Height = msg.Width, msg.Height
	case frameMsg:
		return a.frame(msg)
	case spinMsg:
		return a.spin(msg)
	case toastMsg:
		return a.expireToast()
	case core.OpenOverlayMsg:
		if !a.inGate {
			a.overlays = append(a.overlays, msg)
		}
		return nil
	case core.CloseOverlayMsg:
		i := len(a.overlays) - 1
		if msg.Screen != nil {
			i = slices.IndexFunc(a.overlays, func(o core.OpenOverlayMsg) bool { return o.Screen == msg.Screen })
		}
		if i >= 0 {
			a.overlays = slices.Delete(a.overlays, i, i+1)
		}
		return nil
	case core.ToastMsg:
		return a.notify(msg.Text, msg.Tone)
	case core.SwitchTabMsg:
		return a.switchTab(msg)
	default:
		cmd, own := a.handle(msg)
		if own {
			return cmd
		}
		return tea.Batch(cmd, a.broadcast(msg))
	}
	return a.broadcast(msg)
}

func (a *app) handle(msg tea.Msg) (tea.Cmd, bool) {
	st := a.st
	switch msg := msg.(type) {
	case gate.ChooseMsg, gate.SubmitMsg, gate.BackMsg, gate.QuitMsg, gate.SystemAuthMsg, gate.UsePasswordMsg, gate.LoginKeyMsg:
		return a.gateIntent(msg), true
	case maintenanceDoneMsg:
		return a.maintenanceDone(msg), true
	case unlockDoneMsg:
		return a.unlockDone(msg), true
	case loginStartedMsg, loginProgressMsg, loginApprovedMsg, loginDoneMsg, loginCopiedMsg, restoreDoneMsg:
		return a.loginMsg(msg), true
	case identityMsg, syncDoneMsg:
		return a.applyOwn(msg), true
	case idleMsg:
		return a.idle(), true
	case repairCancelMsg:
		return a.pollStatus(0), true
	case core.RequestRepairMsg:
		if st.RepairBusy() || st.Locked {
			clear(msg.Password)
			return nil, true
		}
		return a.startMaintenance(trigDoctor, msg.Password, false, "Fixing issues"), true
	case core.RequestLoginMsg:
		if a.inGate || st.Locked {
			return nil, true
		}
		a.life.loginFromDash = true
		return a.startLogin(), true
	case core.RequestLockMsg:
		return a.requestLock(), true
	case core.RequestSyncMsg:
		return a.requestSync(), true
	case core.RefreshMsg:
		return a.refreshSnapshot(), true
	case core.KeysChangedMsg:
		return tea.Batch(a.loadKeys(false), a.invalidateSigning()), true
	case core.CancelClipMsg:
		if st.Clip != nil && st.Clip.ID < msg.Since {
			st.Clip = nil
		}
		return nil, true
	case core.ClipStartMsg:
		return a.startClip(msg.Clip), true
	case core.ClipTickMsg:
		return a.clipTick(msg.ID), true
	case core.ClipClearedMsg:
		return a.clipCleared(msg), true
	case core.StatusMsg:
		return a.status(msg)
	case core.SnapshotMsg:
		return a.snapshot(msg)
	}
	return a.apply(msg), false
}

func (a *app) input(msg tea.Msg) tea.Cmd {
	st := a.st
	press, isKey := msg.(tea.KeyPressMsg)
	k := ""
	if isKey {
		k = strings.ToLower(press.String())
	}
	if k == "ctrl+c" {
		return a.quit()
	}
	switch {
	case a.floor():
		if k == "q" {
			return a.quit()
		}
		return nil
	case a.inGate:
		return a.gate.Update(msg, st)
	case st.Busy.Clipboard:
		if k == "esc" && len(a.overlays) > 0 {
			return a.updateTop(msg)
		}
		return nil
	case len(a.overlays) > 0:
		return a.updateTop(msg)
	}
	if c, ok := a.screens[a.active].(core.Capturer); !isKey || ok && c.Capturing() {
		return a.updateScreen(msg)
	}
	switch k {
	case "1", "2", "3", "4", "5":
		if i := int(k[0] - '1'); i < len(a.tabs) {
			return a.setTab(a.tabs[i])
		}
		return nil
	case "left", "right":
		i := slices.Index(a.tabs, a.active) - 1
		if k == "right" {
			i += 2
		}
		return a.setTab(a.tabs[max(0, min(i, len(a.tabs)-1))])
	case "m":
		a.openMenu()
		return nil
	case "l":
		return core.Send(core.RequestLockMsg{})
	case "q":
		return a.quit()
	}
	return a.updateScreen(msg)
}

func (a *app) quit() tea.Cmd {
	if a.st.Busy.PasswordChange {
		return a.warn("Warning", "Quit is unavailable until the password change finishes")
	}
	return tea.Quit
}

func (a *app) top() core.Screen {
	if n := len(a.overlays); n > 0 {
		return a.overlays[n-1].Screen
	}
	return a.screens[a.active]
}

func (a *app) updateTop(msg tea.Msg) tea.Cmd {
	o := &a.overlays[len(a.overlays)-1]
	var cmd tea.Cmd
	o.Screen, cmd = o.Screen.Update(msg, a.st)
	return cmd
}

func (a *app) updateScreen(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	a.screens[a.active], cmd = a.screens[a.active].Update(msg, a.st)
	return cmd
}

func (a *app) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	if a.inGate {
		cmds = append(cmds, a.gate.Update(msg, a.st))
	}
	for i := range a.overlays {
		var cmd tea.Cmd
		a.overlays[i].Screen, cmd = a.overlays[i].Screen.Update(msg, a.st)
		cmds = append(cmds, cmd)
	}
	for _, t := range allTabs {
		var cmd tea.Cmd
		a.screens[t], cmd = a.screens[t].Update(msg, a.st)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (a *app) setTab(t core.Tab) tea.Cmd {
	a.active = t
	return a.broadcast(core.SwitchTabMsg{Tab: t})
}

func (a *app) switchTab(msg core.SwitchTabMsg) tea.Cmd {
	if !slices.Contains(a.tabs, msg.Tab) {
		return nil
	}
	if k, ok := a.screens[core.TabKeys].(*keys.Model); ok && msg.Select != "" {
		k.Select(msg.Select)
		a.pendingSelect = ""
		if !slices.ContainsFunc(a.st.Keys, func(s actions.KeySummary) bool { return s.Name == msg.Select }) {
			a.pendingSelect = msg.Select
		}
	}
	return a.setTab(msg.Tab)
}

func (a *app) openMenu() {
	m := newMenu(a.screens[a.active].Actions(a.st))
	w, h := m.Size(a.st, a.st.Width-4, a.st.Height-2)
	bw, bh := core.BodySize(a.st)
	x := (bw - w) / 2
	if a.st.Width >= 60 {
		x = bw - w
	}
	a.overlays = append(a.overlays, core.OpenOverlayMsg{Screen: m, X: x, Y: bh - h, W: w, H: h})
}

func tick(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

func (a *app) wantFrames() bool {
	return a.inGate && a.animate && a.st.Width >= 40 && a.st.Height >= 16
}

func spinning(s core.Screen) bool {
	sp, ok := s.(core.Spinner)
	return ok && sp.Spinning()
}

func (a *app) floor() bool { return a.st.Width < 40 || a.st.Height < 12 }

func (a *app) wantSpin() bool {
	switch {
	case a.floor():
		return false
	case a.inGate:
		return a.gate.Spinning()
	}
	return a.st.Chip().Tone == ui.ToneBusy || spinning(a.screens[a.active]) || len(a.overlays) > 0 && spinning(a.top())
}

func (a *app) timers() tea.Cmd {
	var cmds []tea.Cmd
	if !a.frameOn && a.wantFrames() {
		a.frameOn = true
		a.frameGen++
		cmds = append(cmds, tick(frameEvery, frameMsg{a.frameGen}))
	}
	if !a.spinOn && a.wantSpin() {
		a.spinOn = true
		a.spinGen++
		cmds = append(cmds, tick(spinEvery, spinMsg{a.spinGen}))
	}
	return tea.Batch(cmds...)
}

func (a *app) frame(msg frameMsg) tea.Cmd {
	if msg.gen != a.frameGen {
		return nil
	}
	if !a.wantFrames() {
		a.frameOn = false
		return nil
	}
	a.gate.Tick()
	return tick(frameEvery, msg)
}

func (a *app) spin(msg spinMsg) tea.Cmd {
	if msg.gen != a.spinGen {
		return nil
	}
	if !a.wantSpin() {
		a.spinOn = false
		return nil
	}
	a.st.SpinFrame++
	return tick(spinEvery, msg)
}

func (a *app) shutdown() {
	a.cancel()
	a.dropRestore()
	a.gate.Wipe()
	for _, o := range a.overlays {
		o.Screen.Update(core.LockedMsg{}, a.st)
	}
}
