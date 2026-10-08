package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/screen/health"
	"github.com/itzzritik/forged/cli/internal/tui/screen/keys"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

const statusFailureThreshold = 2

type identityMsg struct {
	id          core.ID
	name, email string
	err         error
}

type syncDoneMsg struct {
	id  core.ID
	err error
}

func (a *app) recount() { a.st.ProblemCount = health.Count(a.st, a.paths) }

func (a *app) assess(id core.ID) tea.Cmd {
	repair := a.st.Deps.Repair
	return func() tea.Msg {
		res, err := repair(readiness.RunOptions{Mode: readiness.ModeAssessOnly})
		if err != nil {
			err = fmt.Errorf("checking this machine: %w", err)
		}
		return core.SnapshotMsg{ID: id, Result: res, Err: err}
	}
}

func (a *app) refreshSnapshot() tea.Cmd {
	if a.st.Busy.Maintenance {
		return nil
	}
	a.life.snapID = core.NextID()
	return a.assess(a.life.snapID)
}

func (a *app) pollStatus(delay time.Duration) tea.Cmd {
	st := a.st
	if !st.Snapshot.VaultExists || st.Recovery() {
		return nil
	}
	id, load := core.NextID(), st.Deps.LoadStatus
	a.life.statusID = id
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		status, err := load()
		if err != nil {
			err = fmt.Errorf("loading daemon status: %w", err)
		}
		return core.StatusMsg{ID: id, Status: status, Err: err}
	})
}

func (a *app) loadKeys(local bool) tea.Cmd {
	id, list, listLocal := core.NextID(), a.st.Deps.ListKeys, a.st.Deps.ListLocalKeys
	return func() tea.Msg {
		if local {
			if ks, err := listLocal(); err == nil {
				return core.KeysMsg{ID: id, Keys: ks}
			}
		}
		ks, err := list()
		if err != nil {
			err = fmt.Errorf("loading keys: %w", err)
		}
		return core.KeysMsg{ID: id, Keys: ks, Err: err}
	}
}

func (a *app) loadSigning() tea.Cmd {
	if a.st.Recovery() || a.life.signingLoading {
		return nil
	}
	a.life.signingPending, a.life.signingLoading = false, true
	id, load := core.NextID(), a.st.Deps.LoadSigningStatus
	a.life.signingLoad = id
	return func() tea.Msg {
		status, err := load()
		if err != nil {
			err = fmt.Errorf("loading commit signing status: %w", err)
		}
		return core.SigningMsg{ID: id, Status: status, Err: err}
	}
}

func (a *app) invalidateSigning() tea.Cmd {
	a.st.SigningLoaded, a.st.SigningErr = false, ""
	if a.life.signingLoading {
		a.life.signingPending = true
		return nil
	}
	return a.loadSigning()
}

func (a *app) loadAccount() tea.Cmd {
	id, load := core.NextID(), a.st.Deps.LoadCredentials
	a.life.identityID = id
	return func() tea.Msg {
		creds, err := load()
		if err != nil {
			return identityMsg{id: id, err: fmt.Errorf("loading account identity: %w", err)}
		}
		return identityMsg{id: id, name: strings.TrimSpace(creds.Name), email: strings.TrimSpace(creds.Email)}
	}
}

func (a *app) requestSync() tea.Cmd {
	st := a.st
	switch {
	case a.inGate:
		return nil
	case st.SyncIssue() != "":
		return core.Send(core.SwitchTabMsg{Tab: core.TabHealth})
	case !st.Snapshot.LoggedIn || st.CredentialError() != "":
		if st.Busy.Maintenance {
			return nil
		}
		a.life.loginFromDash = true
		return a.startLogin()
	case st.Busy.Sync:
		return nil
	}
	st.Busy.Sync = true
	id, trigger := core.NextID(), st.Deps.TriggerSync
	a.life.syncID = id
	return func() tea.Msg { return syncDoneMsg{id: id, err: trigger()} }
}

func (a *app) applyOwn(msg tea.Msg) tea.Cmd {
	st := a.st
	switch msg := msg.(type) {
	case identityMsg:
		if msg.id != a.life.identityID || !st.Snapshot.LoggedIn {
			return nil
		}
		if msg.err != nil {
			st.Reporter.Report("app", "load account identity", msg.err)
			return nil
		}
		st.AccountName, st.AccountEmail = msg.name, msg.email
	case syncDoneMsg:
		if msg.id != a.life.syncID {
			return nil
		}
		st.Busy.Sync = false
		var cmd tea.Cmd
		if msg.err != nil {
			if diag := strings.TrimSpace(ui.Sanitize(msg.err.Error())); accountauth.IsCredentialDiagnostic(diag) {
				st.Snapshot.LoginCheckError = diag
				st.Reporter.Report("app", "sync.trigger", fmt.Errorf("syncing vault: %w", msg.err))
				a.recount()
			} else {
				cmd = a.notify(st.Reporter.Report("app", "sync.trigger", fmt.Errorf("syncing vault: %w", msg.err)), ui.ToneBad)
			}
		}
		return tea.Batch(cmd, a.pollStatus(0))
	}
	return nil
}

func (a *app) status(msg core.StatusMsg) (tea.Cmd, bool) {
	if msg.ID != a.life.statusID {
		return nil, true
	}
	st := a.st
	wasUnlocked := st.Status.SensitiveKnown && st.Status.Unlocked
	wasPending, had := st.SyncPending(), st.StatusLoaded
	pull, push := st.Status.LastSuccessfulPullAt, st.Status.LastSuccessfulPushAt
	refreshHealth, refreshCred := false, false
	if msg.Err == nil {
		refreshHealth = st.StatusDown
		a.life.statusFailures, st.StatusDown = 0, false
		s := msg.Status
		s.Error = strings.TrimSpace(ui.Sanitize(s.Error))
		if !s.SensitiveReported {
			s.Unlocked, s.SensitiveKnown = st.Status.Unlocked, st.Status.SensitiveKnown
		}
		if s.Error != "" && s.Error != st.Status.Error {
			st.Reporter.Report("app", "sync.status", errors.New(s.Error))
		}
		st.Status, st.StatusLoaded = s, true
		refreshCred = a.life.refreshAfterUnlock && !st.Busy.Maintenance && st.Snapshot.VaultExists
		if refreshCred {
			a.life.refreshAfterUnlock = false
		}
	} else {
		st.Reporter.Report("app", "runtime.status", msg.Err)
		if a.life.statusFailures++; a.life.statusFailures >= statusFailureThreshold {
			refreshHealth = !st.StatusDown
			st.StatusDown = true
		}
		st.Status.Syncing = false
	}
	if refreshHealth && st.Health != core.HealthFixing {
		st.Health = core.HealthChecking
	}
	var cmds []tea.Cmd
	if refreshHealth || refreshCred {
		cmds = append(cmds, a.refreshSnapshot())
	}
	cmds = append(cmds, a.sessionLoss(wasUnlocked))
	if st.Snapshot.VaultExists {
		cmds = append(cmds, a.pollStatus(time.Second))
		synced := had && (st.Status.LastSuccessfulPullAt.After(pull) || st.Status.LastSuccessfulPushAt.After(push))
		if msg.Err == nil && (synced || wasPending && !st.SyncPending()) {
			cmds = append(cmds, a.invalidateSigning())
		}
	}
	a.recount()
	return tea.Batch(cmds...), false
}

func (a *app) snapshot(msg core.SnapshotMsg) (tea.Cmd, bool) {
	st := a.st
	if msg.ID == a.life.bootID && !a.life.booted {
		if msg.Err == nil {
			st.Snapshot, st.Summary, st.Health = msg.Result.Snapshot, msg.Result.Summary, core.HealthOK
		}
		cmd := a.assessed(msg)
		a.recount()
		return cmd, false
	}
	if st.Busy.Maintenance || msg.ID != a.life.snapID {
		return nil, true
	}
	var cmd tea.Cmd
	if msg.Err != nil {
		st.Health = core.HealthBad
		cmd = a.notify("Health check failed: "+st.Reporter.Report("app", "snapshot.refresh", msg.Err), ui.ToneBad)
	} else {
		wasRecovery := st.Recovery()
		st.Snapshot, st.Health = msg.Result.Snapshot, core.HealthOK
		if ready(st.Snapshot) {
			st.RepairErr = ""
		}
		switch rec := st.Recovery(); {
		case rec && !wasRecovery:
			cmd = a.enterRecovery()
		case !rec && wasRecovery:
			cmd = a.exitRecovery()
		}
	}
	a.recount()
	return cmd, false
}

func newer(last *core.ID, id core.ID) bool {
	if id < *last {
		return false
	}
	*last = id
	return true
}

func current(ks []actions.KeySummary, name, fingerprint string) bool {
	i := slices.IndexFunc(ks, func(k actions.KeySummary) bool { return k.Name == name })
	return i >= 0 && (ks[i].Fingerprint == "" || ks[i].Fingerprint == fingerprint)
}

func (a *app) apply(msg tea.Msg) tea.Cmd {
	st := a.st
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case core.KeysMsg:
		if !newer(&a.life.keysID, msg.ID) {
			return nil
		}
		if msg.Err != nil {
			st.KeysErr = st.Reporter.Report("app", "load keys", msg.Err)
			return nil
		}
		st.Keys, st.KeysLoaded, st.KeysErr = msg.Keys, true, ""
		for name, d := range st.Details {
			if !current(st.Keys, name, d.Fingerprint) {
				delete(st.Details, name)
			}
		}
		// Select waits for the reloaded list; the Keys screen drops a name it cannot find.
		if k, ok := a.screens[core.TabKeys].(*keys.Model); ok && a.pendingSelect != "" {
			k.Select(a.pendingSelect)
		}
		a.pendingSelect = ""
		return nil
	case core.DetailMsg:
		if msg.Err == nil && current(st.Keys, msg.Name, msg.Detail.Fingerprint) {
			st.Details[msg.Name] = msg.Detail
		}
		return nil
	case core.SigningMsg:
		if msg.ID == a.life.signingLoad && a.life.signingLoading {
			a.life.signingLoading = false
			if a.life.signingPending {
				return a.loadSigning()
			}
		}
		if !newer(&a.life.signingID, msg.ID) {
			return nil
		}
		switch {
		case msg.Err == nil:
			st.Signing, st.SigningLoaded, st.SigningErr = msg.Status, true, ""
		case msg.ID == a.life.signingLoad:
			st.SigningLoaded = true
			st.SigningErr = st.Reporter.Report("app", "load commit signing status", msg.Err)
		case msg.ID > a.life.signingLoad:
			// A failed screen mutation newer than the last root load; older errors defer to that load.
			cmd = a.invalidateSigning()
		}
	case core.SecurityMsg:
		if !newer(&a.life.securityID, msg.ID) {
			return nil
		}
		if msg.Err != nil {
			st.SecurityErr = st.Reporter.Report("app", "load security settings", msg.Err)
		} else {
			st.Security, st.SecurityLoaded, st.SecurityErr = msg.State, true, ""
		}
	default:
		return nil
	}
	a.recount()
	return cmd
}
