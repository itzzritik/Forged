package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/gate"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type loginStartedMsg struct {
	id      core.ID
	ctx     context.Context
	session actions.LoginSession
	err     error
}

type loginProgressMsg struct {
	id       core.ID
	progress actions.LoginProgress
}

type loginApprovedMsg struct {
	id    core.ID
	creds actions.AccountCredentials
}

type loginDoneMsg struct {
	id                   core.ID
	creds                actions.AccountCredentials
	err                  error
	canceled, committing bool
}

type loginCopiedMsg struct {
	id, since core.ID
	err       error
}

type restoreDoneMsg struct {
	id  core.ID
	pw  *pwBuffer
	err error
}

type pwBuffer struct {
	mu    sync.Mutex
	value []byte
}

func newPWBuffer(pw []byte) *pwBuffer {
	b := &pwBuffer{value: append([]byte(nil), pw...)}
	clear(pw)
	return b
}

func (b *pwBuffer) use(fn func([]byte) error) error {
	b.mu.Lock()
	if len(b.value) == 0 {
		b.mu.Unlock()
		return errors.New("password is unavailable")
	}
	working := append([]byte(nil), b.value...)
	b.mu.Unlock()
	defer clear(working)
	return fn(working)
}

func (b *pwBuffer) take() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	v := b.value
	b.value = nil
	return v
}

func (b *pwBuffer) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.value)
	b.value = nil
}

func (a *app) onLogin() bool { return a.inGate && a.gate.Kind() == gate.Login }

func (a *app) stopLogin() {
	if c := a.life.loginCancel; c != nil {
		c()
		a.life.loginCancel = nil
	}
}

func (a *app) cancelLogin() {
	a.life.loginID = core.NextID()
	a.life.loginProgress = nil
	a.life.loginCommitting, a.life.loginCopying = false, false
	a.stopLogin()
}

func (a *app) loginToHealth() tea.Cmd {
	if !a.life.opened && !a.life.loginFromDash {
		return tea.Batch(a.notify("Sync needs attention. Open Health to review it.", ui.ToneWarn), a.finishBoot())
	}
	var cmd tea.Cmd
	if a.onLogin() {
		cmd = a.leaveGate()
	}
	return tea.Batch(cmd, core.Send(core.SwitchTabMsg{Tab: core.TabHealth}))
}

func (a *app) setLoginStatus(status string) {
	a.gate.SetLogin(a.life.loginCode, a.life.loginURL, status, a.life.loginCommitting)
}

func (a *app) loginFail(text string) tea.Cmd {
	a.life.loginFailed = true
	a.setLoginStatus("")
	a.gate.SetError(text)
	return nil
}

func (a *app) startLogin() tea.Cmd {
	st := a.st
	a.cancelLogin()
	if st.SyncIssue() != "" {
		return a.loginToHealth()
	}
	a.showGate(gate.Login)
	a.life.loginCode, a.life.loginURL, a.life.loginFailed = "", "", false
	a.setLoginStatus("Opening approval link")
	id := core.NextID()
	a.life.loginID = id
	ctx, cancel := context.WithCancel(a.ctx)
	a.life.loginCancel = cancel
	ch := make(chan actions.LoginProgress, 8)
	a.life.loginProgress = ch
	start, server := st.Deps.StartLogin, st.Deps.DefaultServer
	return tea.Batch(waitProgress(id, ch), func() tea.Msg {
		s, err := start(ctx, server, func(p actions.LoginProgress) {
			select {
			case ch <- p:
			default:
			}
		})
		close(ch)
		if err != nil {
			err = fmt.Errorf("starting log-in: %w", err)
		}
		return loginStartedMsg{id: id, ctx: ctx, session: s, err: err}
	})
}

func waitProgress(id core.ID, ch <-chan actions.LoginProgress) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return loginProgressMsg{id: id, progress: p}
	}
}

func waitLogin(ctx context.Context, id core.ID, s actions.LoginSession) tea.Cmd {
	return func() tea.Msg {
		creds, err := s.Wait(ctx)
		switch {
		case errors.Is(err, context.Canceled):
			return loginDoneMsg{id: id, canceled: true}
		case err != nil:
			return loginDoneMsg{id: id, err: fmt.Errorf("waiting for approval: %w", err)}
		}
		return loginApprovedMsg{id: id, creds: creds}
	}
}

func (a *app) openLoginURL() error {
	url := strings.TrimSpace(a.life.loginURL)
	if url == "" {
		return nil
	}
	if err := a.st.Deps.OpenLink(url); err != nil {
		return fmt.Errorf("opening approval link: %w", err)
	}
	return nil
}

func loginWarning(err error) string {
	syncPending := errors.Is(err, actions.ErrAccountChangeSyncCleanupPending)
	secretPending := errors.Is(err, actions.ErrAccountChangeCredentialSecretCleanupPending)
	switch {
	case syncPending && secretPending:
		return "Account saved, but Forged could not remove prior sync state or a retired credential artifact. Sync is paused to protect it; resolve both before switching accounts."
	case syncPending:
		return "Account saved, but Forged could not remove prior sync state. Sync is paused to protect it. Open Health to review it."
	case secretPending:
		return "Account saved, but a prior local credential artifact could not be removed. It will not be retried automatically; resolve it before switching accounts."
	case errors.Is(err, actions.ErrAccountChangeCommittedUnconfirmed):
		return "Account saved, but Forged could not confirm local cleanup. Review it before switching accounts."
	}
	return ""
}

func (a *app) loginMsg(msg tea.Msg) tea.Cmd {
	st := a.st
	switch msg := msg.(type) {
	case loginProgressMsg:
		if msg.id != a.life.loginID || !a.onLogin() || a.life.loginProgress == nil || a.life.loginCommitting {
			return nil
		}
		if s := strings.TrimSpace(msg.progress.Status); s != "" {
			a.setLoginStatus(s)
			a.gate.SetError("")
		}
		return waitProgress(msg.id, a.life.loginProgress)
	case loginStartedMsg:
		if msg.id != a.life.loginID || !a.onLogin() {
			return nil
		}
		if st.SyncIssue() != "" {
			a.cancelLogin()
			return a.loginToHealth()
		}
		a.life.loginProgress = nil
		if msg.err != nil {
			a.stopLogin()
			return a.loginFail(st.Reporter.Report("app", "login.start", msg.err))
		}
		a.life.loginCode, a.life.loginURL = msg.session.VerificationCode, msg.session.URL
		status := ""
		if a.st.Deps.CanOpenLinks {
			if err := a.openLoginURL(); err != nil {
				st.Reporter.Report("app", "browser.open", err)
				status = "Couldn't open the link. Press c to copy it."
			}
		}
		a.setLoginStatus(status)
		return waitLogin(msg.ctx, msg.id, msg.session)
	case loginApprovedMsg:
		if msg.id != a.life.loginID || !a.onLogin() {
			return nil
		}
		if st.SyncIssue() != "" {
			a.cancelLogin()
			return a.loginToHealth()
		}
		a.life.loginCopying = false
		a.stopLogin()
		a.life.loginCommitting = true
		a.gate.SetError("")
		a.setLoginStatus("Saving account securely")
		id, creds, save := msg.id, msg.creds, st.Deps.SaveCredentials
		return func() tea.Msg {
			err := save(creds)
			if err != nil {
				err = fmt.Errorf("saving account: %w", err)
			}
			return loginDoneMsg{id: id, creds: creds, err: err, committing: true}
		}
	case loginDoneMsg:
		if msg.id != a.life.loginID || !a.onLogin() || msg.committing != a.life.loginCommitting {
			return nil
		}
		a.life.loginCommitting = false
		a.stopLogin()
		if msg.canceled {
			return nil
		}
		return a.loggedIn(msg)
	case loginCopiedMsg:
		if msg.id != a.life.loginID || !a.onLogin() || a.life.loginCommitting {
			st.Reporter.Report("app", "clipboard.copy", msg.err)
			return nil
		}
		a.life.loginCopying = false
		if msg.err != nil {
			st.Reporter.Report("app", "clipboard.copy", msg.err)
			a.setLoginStatus("Couldn't copy the link. Select it below instead.")
			return nil
		}
		a.setLoginStatus("Approval link copied")
		return core.Send(core.CancelClipMsg{Since: msg.since})
	case restoreDoneMsg:
		return a.restored(msg)
	}
	return nil
}

func (a *app) loggedIn(msg loginDoneMsg) tea.Cmd {
	st := a.st
	warn := loginWarning(msg.err)
	if msg.err != nil && warn == "" {
		return a.loginFail(st.Reporter.Report("app", "login.finish", msg.err))
	}
	st.Snapshot.LoggedIn, st.Snapshot.LoginCheckError = true, ""
	if accountauth.IsCredentialDiagnostic(st.Status.Error) {
		st.Status.Error = ""
	}
	st.AccountEmail, st.AccountName = strings.TrimSpace(msg.creds.Email), strings.TrimSpace(msg.creds.Name)
	a.life.identityID = core.NextID()
	a.recount()
	if st.Locked {
		return a.post(notice{title: "Signed in", text: warn, tone: ui.ToneWarn})
	}
	if !a.life.opened && st.Snapshot.VaultExists {
		cmd := a.post(notice{title: "Signed in", text: warn, tone: ui.ToneWarn})
		a.showGate(gate.Busy)
		a.gate.SetBusy("Finishing account setup")
		return tea.Batch(cmd, a.startMaintenance(trigPostLogin, nil, false, "Finishing account setup"))
	}
	if warn != "" {
		if st.Snapshot.VaultExists {
			return tea.Batch(a.leaveGate(), a.warn("Signed in", warn))
		}
		a.welcome()
		return a.warn("Signed in", warn)
	}
	if !st.Snapshot.VaultExists {
		a.showGate(gate.Restore)
		return nil
	}
	return tea.Batch(a.leaveGate(), a.startMaintenance(trigPostLogin, nil, false, "Finishing account setup"))
}

func (a *app) loginKey(k string) tea.Cmd {
	if !a.onLogin() || a.life.loginCommitting {
		return nil
	}
	switch k {
	case "esc":
		a.cancelLogin()
		if a.life.loginFromDash {
			return a.leaveGate()
		}
		a.welcome()
	case "c":
		if !a.st.Deps.CanCopy {
			return nil
		}
		return a.copyLoginURL()
	case "enter":
		if a.life.loginFailed {
			return a.startLogin()
		}
		if a.life.loginURL == "" {
			return nil
		}
		if !a.st.Deps.CanOpenLinks {
			if !a.st.Deps.CanCopy {
				return nil
			}
			return a.copyLoginURL()
		}
		if err := a.openLoginURL(); err != nil {
			a.st.Reporter.Report("app", "browser.open", err)
			a.setLoginStatus("Couldn't open the link. Press c to copy it.")
			return nil
		}
		a.setLoginStatus("")
	}
	return nil
}

func (a *app) copyLoginURL() tea.Cmd {
	if a.life.loginCopying || a.life.loginURL == "" || a.life.loginFailed {
		return nil
	}
	a.life.loginCopying = true
	a.setLoginStatus("Copying approval link")
	id, since, url, copyText := a.life.loginID, core.NextID(), a.life.loginURL, a.st.Deps.CopyText
	return func() tea.Msg {
		return core.Copy(copyText, url, func(err error) tea.Msg {
			if err != nil {
				err = fmt.Errorf("copying approval link: %w", err)
			}
			return loginCopiedMsg{id: id, since: since, err: err}
		})
	}
}

func (a *app) restore(pw []byte) tea.Cmd {
	a.dropRestore()
	a.gate.SetBusy("Decrypting vault")
	id, run := core.NextID(), a.st.Deps.RestoreVault
	a.life.restoreID = id
	buf := newPWBuffer(pw)
	a.life.restorePW = buf
	return func() tea.Msg {
		err := buf.use(run)
		if err != nil {
			buf.clear()
			err = fmt.Errorf("restoring vault: %w", err)
		}
		return restoreDoneMsg{id: id, pw: buf, err: err}
	}
}

func (a *app) dropRestore() {
	a.life.restoreID = core.NextID()
	if b := a.life.restorePW; b != nil {
		b.clear()
		a.life.restorePW = nil
	}
}

func (a *app) restored(msg restoreDoneMsg) tea.Cmd {
	if msg.id != a.life.restoreID || !a.inGate || a.gate.Kind() != gate.Restore {
		msg.pw.clear()
		return nil
	}
	a.life.restorePW = nil
	a.gate.SetBusy("")
	if msg.err != nil {
		text := a.st.Reporter.Report("app", "vault.restore", msg.err)
		msg.pw.clear()
		switch {
		case errors.Is(msg.err, readiness.ErrInvalidRestorePassword):
			text = "Couldn't decrypt vault, incorrect password"
		case errors.Is(msg.err, readiness.ErrNoRemoteLinkedVault):
			text = "No linked vault was found for this account."
		case errors.Is(msg.err, readiness.ErrRestoreTargetExists):
			text = "A local vault was created while restoring. It was kept; reopen Forged."
		}
		a.gate.SetError(text)
		return nil
	}
	return a.startMaintenance(trigUnlock, msg.pw.take(), false, "Setting up Forged")
}
