package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/gate"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const idleAfter = 4 * time.Minute

type idleMsg struct{}

func (a *app) trackIdle() bool {
	st := a.st
	return a.life.booted && st.Snapshot.VaultExists && st.Status.SensitiveKnown && st.Status.Unlocked && !a.inGate && !st.Locked
}

func (a *app) touchIdle() tea.Cmd {
	if !a.trackIdle() {
		a.life.idleDeadline = time.Time{}
		return nil
	}
	a.life.idleDeadline = time.Now().Add(idleAfter)
	return a.armIdle()
}

func (a *app) armIdle() tea.Cmd {
	if a.life.idleArmed || a.life.idleDeadline.IsZero() {
		return nil
	}
	a.life.idleArmed = true
	return tick(max(0, time.Until(a.life.idleDeadline)), idleMsg{})
}

func (a *app) idle() tea.Cmd {
	a.life.idleArmed = false
	if !a.trackIdle() || a.life.idleDeadline.IsZero() {
		a.life.idleDeadline = time.Time{}
		return nil
	}
	if time.Until(a.life.idleDeadline) > 0 {
		return a.armIdle()
	}
	return a.lock()
}

func (a *app) requestLock() tea.Cmd {
	if a.inGate || !a.st.Snapshot.VaultExists {
		return nil
	}
	return a.lock()
}

func (a *app) lock() tea.Cmd {
	a.st.Locked = true
	a.life.idleDeadline, a.life.identityID = time.Time{}, core.NextID()
	a.cancelLogin()
	a.invalidateUnlock()
	cmd := a.closeAll()
	a.showGate(gate.Unlock)
	a.life.repairWall = false
	prompt := "Enter your master password to continue using Forged."
	if a.gate.Trusted() {
		prompt = "Please authenticate to continue using Forged."
	}
	a.gate.SetUnlock(false, prompt)
	return tea.Batch(cmd, a.clearClipForLock())
}

func (a *app) closeAll() tea.Cmd {
	cmd := a.broadcast(core.LockedMsg{})
	a.gate.Wipe()
	a.overlays = nil
	return cmd
}

func (a *app) sessionLoss(wasUnlocked bool) tea.Cmd {
	st := a.st
	if !st.Status.SensitiveKnown || st.Status.Unlocked {
		return nil
	}
	if wasUnlocked && st.Snapshot.VaultExists && (!a.inGate || a.onLogin()) {
		return a.lock()
	}
	return a.clearClipForLock()
}

func (a *app) enterRecovery() tea.Cmd {
	st := a.st
	a.cancelLogin()
	st.StatusLoaded, st.Health = false, core.HealthBad
	cmd := a.closeAll()
	a.tabs, a.active = st.Tabs(), core.TabHealth
	if a.inGate && a.gate.Kind() != gate.Unlock {
		return tea.Batch(cmd, a.leaveGate())
	}
	return tea.Batch(cmd, a.setTab(core.TabHealth))
}

func (a *app) exitRecovery() tea.Cmd {
	if !a.life.opened && !a.inGate {
		return a.restart()
	}
	a.tabs = a.st.Tabs()
	return tea.Batch(a.pollStatus(0), a.invalidateSigning(), core.LoadSecurityCmd(a.st))
}

func (a *app) startClip(c *core.Clip) tea.Cmd {
	if c == nil {
		return nil
	}
	st := a.st
	st.Clip, a.life.clipRetries = c, 0
	if st.Locked || st.Status.SensitiveKnown && !st.Status.Unlocked {
		c.Until, c.Clearing = time.Now(), true
		return core.ClipClear(c)
	}
	return core.ClipTick(c.ID)
}

func (a *app) clipTick(id core.ID) tea.Cmd {
	c := a.st.Clip
	if c == nil || c.ID != id || c.Clearing {
		return nil
	}
	if time.Now().Before(c.Until) {
		return core.ClipTick(id)
	}
	c.Clearing = true
	return core.ClipClear(c)
}

// clearClipForLock is idempotent: once the deadline has passed, the countdown or its retry owns the clear.
func (a *app) clearClipForLock() tea.Cmd {
	c := a.st.Clip
	if c == nil || c.Clearing || !c.Until.After(time.Now()) {
		return nil
	}
	c.ID, c.Until, c.Clearing = core.NextID(), time.Now(), true
	return core.ClipClear(c)
}

func clipBackoff(tries int) time.Duration {
	if tries >= 3 {
		return 30 * time.Second
	}
	return 5 * time.Second
}

func (a *app) clipCleared(msg core.ClipClearedMsg) tea.Cmd {
	st := a.st
	c := st.Clip
	if c == nil || c.ID != msg.ID {
		return nil
	}
	c.Clearing = false
	if msg.Err != nil {
		st.Reporter.Report("app", "clipboard.clear-private", msg.Err)
		a.life.clipRetries++
		var t tea.Cmd
		if a.life.clipRetries == 1 {
			t = a.notify("Couldn't clear the clipboard", ui.ToneBad)
		}
		return tea.Batch(t, tick(clipBackoff(a.life.clipRetries), core.ClipTickMsg{ID: c.ID}))
	}
	st.Clip = nil
	if msg.Cleared {
		return a.notify("Private key cleared from clipboard", ui.ToneGood)
	}
	return a.notify("Clipboard changed, nothing to clear", ui.ToneWarn)
}
