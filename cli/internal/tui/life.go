package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/gate"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const (
	trigBoot      = "boot"
	trigSetup     = "setup"
	trigUnlock    = "unlock"
	trigPostLogin = "post-login"
	trigDoctor    = "doctor"
)

type life struct {
	booted, opened, unlockPending, repairWall bool

	bootID, maintID, statusID, snapID          core.ID
	unlockID, loginID, restoreID, syncID       core.ID
	identityID                                 core.ID
	signingID, signingLoad, securityID, keysID core.ID
	signingLoading, signingPending             bool
	statusFailures                             int
	refreshAfterUnlock, offerHeadless          bool
	unlockCancel, loginCancel                  context.CancelFunc
	loginProgress                              <-chan actions.LoginProgress
	loginCommitting, loginCopying              bool
	loginFailed, loginFromDash                 bool
	loginCode, loginURL                        string
	restorePW                                  *pwBuffer
	idleDeadline                               time.Time
	idleArmed                                  bool
	clipRetries                                int
}

type maintenanceDoneMsg struct {
	id                     core.ID
	trigger                string
	result                 readiness.RunResult
	err, unlockErr         error
	passwordUsed, unlocked bool
}

type unlockDoneMsg struct {
	id       core.ID
	result   actions.UnlockResult
	err      error
	password bool
}

func ready(s readiness.Snapshot) bool {
	return s.State == readiness.StateReady || s.State == readiness.StateReadyEmpty
}

func credentialVisible(st *core.State) bool {
	return strings.TrimSpace(st.Snapshot.LoginCheckError) != "" ||
		st.StatusLoaded && accountauth.IsCredentialDiagnostic(strings.TrimSpace(st.Status.Error))
}

func (a *app) showGate(k gate.Kind) {
	a.inGate = true
	a.gate.Show(k)
	a.gate.Refresh(a.st)
}

func (a *app) leaveGate() tea.Cmd {
	a.inGate, a.frameOn = false, false
	a.frameGen++
	a.gate.Wipe()
	return tea.Batch(a.setTab(a.active), a.touchIdle(), a.flush())
}

func (a *app) startBoot() tea.Cmd {
	a.life.booted, a.life.opened = false, false
	a.st.Health = core.HealthChecking
	a.showGate(gate.Launch)
	a.life.bootID = core.NextID()
	return a.assess(a.life.bootID)
}

func (a *app) restart() tea.Cmd {
	st := a.st
	st.StatusLoaded = false
	st.SigningLoaded, st.SigningErr = false, ""
	a.life.signingLoading, a.life.signingPending = false, false
	a.life.signingID = core.NextID()
	return a.startBoot()
}

func (a *app) assessed(msg core.SnapshotMsg) tea.Cmd {
	st := a.st
	a.life.booted, a.life.unlockPending = true, false
	if msg.Err != nil {
		st.Health = core.HealthBad
		text := st.Reporter.Report("app", "startup.assess", msg.Err)
		if a.intent.Doctor {
			return tea.Batch(a.leaveGate(), a.notify(text, ui.ToneBad), core.LoadSecurityCmd(st), a.invalidateSigning())
		}
		a.showGate(gate.Welcome)
		a.gate.SetError(text)
		return nil
	}
	st.StatusLoaded = false
	if st.Recovery() {
		return a.enterRecovery()
	}
	a.tabs = st.Tabs()
	if !st.Snapshot.VaultExists {
		return a.onboarding()
	}
	st.Health = core.HealthChecking
	a.life.unlockPending = true
	if ready(st.Snapshot) && st.Snapshot.IPCSocketReady {
		return a.startStartupUnlock()
	}
	return a.startMaintenance(trigBoot, nil, false, "Checking this machine")
}

func (a *app) onboarding() tea.Cmd {
	st := a.st
	st.Summary, st.Health = readiness.RepairSummary{}, core.HealthOK
	if a.intent.Doctor {
		return tea.Batch(a.leaveGate(), core.LoadSecurityCmd(st), a.invalidateSigning())
	}
	a.welcome()
	return nil
}

func (a *app) welcome() {
	if e := a.st.CredentialError(); e != "" {
		a.showGate(gate.RepairAccount)
		a.gate.SetError(e)
		return
	}
	a.showGate(gate.Welcome)
}

func (a *app) startMaintenance(trigger string, pw []byte, createFirst bool, title string) tea.Cmd {
	st := a.st
	if st.Busy.Maintenance {
		clear(pw)
		return nil
	}
	if trigger == trigDoctor {
		st.RepairErr = ""
	}
	a.life.statusID, a.life.snapID = core.NextID(), core.NextID()
	a.life.statusFailures, st.StatusDown = 0, false
	st.Health = core.HealthFixing
	st.Busy.Maintenance = true
	if a.inGate {
		a.gate.SetBusy(title)
	}
	id := core.NextID()
	a.life.maintID = id
	mode := readiness.ModeInteractiveLauncher
	if trigger == trigDoctor {
		mode = readiness.ModeInteractiveDoctor
	}
	repair, create, unlock, ctx := st.Deps.Repair, st.Deps.CreateVault, st.Deps.UnlockSensitiveLaunch, a.ctx
	pwCopy := append([]byte(nil), pw...)
	clear(pw)
	return func() tea.Msg {
		defer clear(pwCopy)
		msg := maintenanceDoneMsg{id: id, trigger: trigger, passwordUsed: len(pwCopy) > 0}
		if createFirst {
			if err := create(pwCopy); err != nil {
				msg.err = fmt.Errorf("creating vault: %w", err)
				return msg
			}
		}
		opts := readiness.RunOptions{Mode: mode}
		if len(pwCopy) > 0 {
			opts.PromptPassword = func(string) ([]byte, error) { return append([]byte(nil), pwCopy...), nil }
		}
		msg.result, msg.err = repair(opts)
		if msg.err != nil {
			msg.err = fmt.Errorf("repairing this machine: %w", msg.err)
			return msg
		}
		if len(pwCopy) == 0 || !msg.result.Snapshot.VaultExists || msg.result.Next == readiness.NextActionNeedsPassword ||
			trigger != trigSetup && trigger != trigUnlock {
			return msg
		}
		res, err := unlock(ctx, pwCopy, false)
		switch {
		case err != nil:
			msg.unlockErr = fmt.Errorf("unlocking Forged: %w", err)
		case res.PasswordRequired:
			msg.unlockErr = errors.New("startup authentication still required")
		default:
			msg.unlocked = true
		}
		return msg
	}
}

func (a *app) maintenanceDone(msg maintenanceDoneMsg) tea.Cmd {
	if msg.id != a.life.maintID {
		return nil
	}
	// Maintenance cancels the status poll; every outcome must restart it or lock detection stops.
	return tea.Batch(a.maintained(msg), a.pollStatus(0))
}

func (a *app) maintained(msg maintenanceDoneMsg) tea.Cmd {
	st := a.st
	credVisible, email, wasRecovery := credentialVisible(st), st.AccountEmail, st.Recovery()
	action := "maintenance." + msg.trigger
	errText := st.Reporter.Report("app", action, msg.err)
	st.Reporter.Report("app", action+".unlock", msg.unlockErr)
	st.Snapshot, st.Summary, st.Health = msg.result.Snapshot, msg.result.Summary, core.HealthOK
	if msg.err == nil && ready(st.Snapshot) {
		st.RepairErr = ""
	}
	st.Busy.Maintenance = false
	if msg.unlocked {
		st.Status.Unlocked, st.Status.SensitiveKnown = true, true
		if credVisible {
			a.life.refreshAfterUnlock = true
		}
	}
	var account tea.Cmd
	if st.Snapshot.LoggedIn {
		account = a.loadAccount()
	} else {
		st.AccountName, st.AccountEmail = "", ""
	}
	a.recount()
	if msg.err != nil {
		if msg.trigger == trigDoctor {
			st.RepairErr = errText
		}
		st.Health = core.HealthBad
	}
	switch rec := st.Recovery(); {
	case rec && !wasRecovery:
		account = tea.Batch(account, a.enterRecovery())
	case !rec && wasRecovery && !a.life.opened && !a.inGate:
		return tea.Batch(account, a.restart())
	case !rec && wasRecovery:
		a.tabs = st.Tabs()
	}
	if st.Locked {
		return account
	}
	if a.inGate {
		a.gate.SetBusy("")
	}
	if msg.err != nil {
		return tea.Batch(account, a.maintenanceFailed(msg, errText))
	}
	return tea.Batch(account, a.maintenanceNext(msg, email))
}

func (a *app) maintenanceFailed(msg maintenanceDoneMsg, errText string) tea.Cmd {
	st := a.st
	switch {
	case a.inGate && (msg.trigger == trigSetup || msg.passwordUsed):
		a.gate.SetError(errText)
	case msg.trigger == trigBoot && st.Snapshot.VaultExists && a.life.unlockPending:
		return tea.Batch(a.notify(errText, ui.ToneBad), a.startStartupUnlock())
	case msg.trigger == trigBoot:
		cmd := a.onboarding()
		if a.inGate && a.gate.Kind() == gate.Welcome {
			a.gate.SetError(errText)
			return cmd
		}
		return tea.Batch(cmd, a.notify(errText, ui.ToneBad))
	case msg.trigger == trigPostLogin && !a.life.opened:
		return tea.Batch(a.notify(errText, ui.ToneBad), a.finishBoot())
	case msg.trigger != trigDoctor:
		return a.notify(errText, ui.ToneBad)
	}
	return nil
}

func (a *app) maintenanceNext(msg maintenanceDoneMsg, email string) tea.Cmd {
	st := a.st
	doctor := msg.trigger == trigDoctor
	switch msg.result.Next {
	case readiness.NextActionNeedsCredentialRepair:
		if doctor {
			return tea.Batch(a.notify(st.Snapshot.LoginCheckError, ui.ToneBad), core.LoadSecurityCmd(st))
		}
		a.showGate(gate.RepairAccount)
		a.gate.SetError(strings.TrimSpace(st.Snapshot.LoginCheckError))
		return nil
	case readiness.NextActionNeedsPassword:
		errText := ""
		if msg.passwordUsed {
			errText = "That password did not unlock this device."
		}
		switch {
		case doctor:
			return a.openRepair(errText)
		case !st.Snapshot.VaultExists && (st.Snapshot.LoggedIn || email != ""):
			a.showGate(gate.Restore)
			a.gate.SetError(errText)
		default:
			a.showGate(gate.Unlock)
			a.life.repairWall = true
			a.gate.SetError(errText)
		}
		return nil
	case readiness.NextActionNeedsInteractiveSetup:
		text := ""
		switch {
		case st.Snapshot.LoggedIn:
			text = "No synced vault was found for this account. Start a new vault on this device."
		case doctor:
			text = "No repairs were made. Create or restore a vault to continue."
		}
		if doctor {
			return tea.Batch(a.warn("Warning", text), core.LoadSecurityCmd(st))
		}
		cmd := a.onboarding()
		return tea.Batch(cmd, a.warn("Warning", text))
	}
	if (msg.trigger == trigSetup || msg.trigger == trigUnlock) && st.Snapshot.VaultExists {
		if msg.unlocked {
			a.life.unlockPending, a.life.opened = false, true
			a.life.offerHeadless = msg.passwordUsed
			return a.finishBoot()
		}
		if msg.unlockErr != nil {
			st.Status.Unlocked, st.Status.SensitiveKnown = false, false
		}
		return a.restart()
	}
	if msg.trigger == trigBoot && st.Snapshot.VaultExists && a.life.unlockPending {
		return a.startStartupUnlock()
	}
	return a.finishBoot()
}

func (a *app) startStartupUnlock() tea.Cmd {
	a.showGate(gate.Unlock)
	a.life.repairWall = false
	a.gate.SetUnlock(!a.st.Deps.Remote, "")
	if a.st.Deps.Remote {
		a.gate.SetBusy("Unlocking Forged")
	}
	return a.unlock(nil)
}

func (a *app) unlock(pw []byte) tea.Cmd {
	a.invalidateUnlock()
	id := core.NextID()
	a.life.unlockID = id
	ctx, cancel := context.WithCancel(a.ctx)
	a.life.unlockCancel = cancel
	// A view locked while the shared session stays active must still re-authenticate.
	force, run := a.st.Locked, a.st.Deps.UnlockSensitiveLaunch
	pwCopy := append([]byte(nil), pw...)
	clear(pw)
	return func() tea.Msg {
		defer cancel()
		defer clear(pwCopy)
		res, err := run(ctx, pwCopy, force)
		if err != nil {
			err = fmt.Errorf("unlocking Forged: %w", err)
		}
		return unlockDoneMsg{id: id, result: res, err: err, password: len(pwCopy) > 0}
	}
}

func (a *app) cancelUnlock() {
	if c := a.life.unlockCancel; c != nil {
		c()
		a.life.unlockCancel = nil
	}
}

func (a *app) invalidateUnlock() {
	a.life.unlockID = core.NextID()
	a.cancelUnlock()
}

func (a *app) unlockDone(msg unlockDoneMsg) tea.Cmd {
	if msg.id != a.life.unlockID || a.life.unlockCancel == nil {
		return nil
	}
	a.cancelUnlock()
	st := a.st
	if !a.inGate || a.gate.Kind() != gate.Unlock || a.life.repairWall {
		return nil
	}
	a.gate.SetBusy("")
	switch {
	case msg.err != nil:
		a.gate.SetUnlock(false, "Enter your master password to open Forged.")
		a.gate.SetError(st.Reporter.Report("app", "vault.unlock", msg.err))
		return nil
	case msg.result.PasswordRequired:
		prompt := strings.TrimSpace(msg.result.Prompt)
		if prompt == "" {
			prompt = "Authentication unavailable. Enter your master password to open Forged."
		}
		a.gate.SetUnlock(false, prompt)
		a.gate.SetError("")
		return nil
	}
	st.Locked = false
	st.Status.Unlocked, st.Status.SensitiveKnown = true, true
	if credentialVisible(st) {
		a.life.refreshAfterUnlock = true
	}
	a.life.unlockPending, a.life.opened = false, true
	a.life.offerHeadless = msg.password
	return a.finishBoot()
}

func (a *app) finishBoot() tea.Cmd {
	st := a.st
	if st.Locked {
		return nil
	}
	if st.Health == core.HealthChecking {
		st.Health = core.HealthOK
	}
	a.life.repairWall = false
	if !st.Snapshot.VaultExists {
		return a.onboarding()
	}
	if !a.life.opened {
		return a.startStartupUnlock()
	}
	cmds := []tea.Cmd{a.leaveGate(), a.pollStatus(0), core.LoadSecurityCmd(st), a.loadKeys(true), a.loadSigning()}
	if st.Snapshot.LoggedIn {
		cmds = append(cmds, a.loadAccount())
	}
	return tea.Batch(cmds...)
}

func (a *app) gateIntent(msg tea.Msg) tea.Cmd {
	if !a.inGate {
		if s, ok := msg.(gate.SubmitMsg); ok {
			clear(s.Password)
			clear(s.Confirm)
		}
		return nil
	}
	switch msg := msg.(type) {
	case gate.ChooseMsg:
		if k := a.gate.Kind(); k != gate.Welcome && k != gate.RepairAccount {
			return nil
		}
		if msg.Login {
			a.life.loginFromDash = false
			return a.startLogin()
		}
		a.showGate(gate.Create)
	case gate.SubmitMsg:
		clear(msg.Confirm)
		if msg.Kind != a.gate.Kind() {
			clear(msg.Password)
			return nil
		}
		switch msg.Kind {
		case gate.Create:
			return a.startMaintenance(trigSetup, msg.Password, true, "Setting up Forged")
		case gate.Restore:
			return a.restore(msg.Password)
		case gate.Unlock:
			if a.life.repairWall {
				return a.startMaintenance(trigUnlock, msg.Password, false, "Unlocking Forged")
			}
			a.gate.SetBusy("Unlocking Forged")
			return a.unlock(msg.Password)
		}
		clear(msg.Password)
	case gate.SystemAuthMsg:
		switch {
		case a.gate.Kind() != gate.Unlock || a.life.unlockCancel != nil:
		case a.life.repairWall:
			a.gate.SetError("Enter your master password")
		default:
			return a.startStartupUnlock()
		}
	case gate.UsePasswordMsg:
		if a.gate.Kind() == gate.Unlock {
			a.invalidateUnlock()
			a.gate.SetUnlock(false, "Enter your master password to open Forged.")
			a.gate.Refresh(a.st)
		}
	case gate.LoginKeyMsg:
		return a.loginKey(msg.Key)
	case gate.BackMsg:
		switch a.gate.Kind() {
		case gate.Restore:
			a.dropRestore()
		case gate.Create:
		default:
			return nil
		}
		a.welcome()
	case gate.QuitMsg:
		return a.quit()
	}
	return nil
}
