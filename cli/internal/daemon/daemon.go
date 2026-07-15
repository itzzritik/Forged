package daemon

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/activity"
	forgedagent "github.com/itzzritik/forged/cli/internal/agent"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Daemon struct {
	// syncTransitionMu serializes account and local sync-state transitions.
	// It is never held by link network work; workers take it before sessionMu.
	syncTransitionMu        sync.Mutex
	sessionMu               sync.Mutex
	paths                   config.Paths
	vault                   *vault.Vault
	keyStore                *vault.KeyStore
	activityLog             *activity.ActivityLog
	agent                   *forgedagent.ForgedAgent
	agentServer             *forgedagent.Server
	ipcServer               *ipc.Server
	syncBus                 *forgedsync.Bus
	syncDraining            *forgedsync.Bus
	authBroker              *sensitiveauth.Broker
	sshRouting              *sshrouting.Manager
	routeService            *sshrouting.Service
	syncGeneration          uint64
	syncPending             bool
	syncSuppressed          bool
	accountTransition       uint64
	activeAccountTransition uint64
	syncInitRun             *syncInitRun
	syncRetryDelay          time.Duration
	syncError               string
	linkRun                 *linkRun
	logger                  *slog.Logger
	stop                    chan struct{}
	stopOnce                sync.Once
	shutdownOnce            sync.Once
}

func New(paths config.Paths) *Daemon {
	return &Daemon{
		paths: paths,
		stop:  make(chan struct{}),
	}
}

func (d *Daemon) Run(password []byte) error {
	if err := d.paths.ValidateRuntimePaths(); err != nil {
		return fmt.Errorf("resolving runtime socket paths: %w", err)
	}
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("Setting up logging: %w", err)
	}

	d.logger.Info("starting forged daemon")

	runtimeLock, lockErr := acquireRuntimeLock(d.paths)
	if lockErr != nil {
		return lockErr
	}
	defer runtimeLock.Close()

	if err := d.cleanStaleState(); err != nil {
		return fmt.Errorf("Cleaning stale state: %w", err)
	}

	defer d.shutdown()

	if platform.SSHRoutingSupported() {
		d.routeService = sshrouting.NewService(d.paths, nil)
		d.routeService.SetOnMutation(d.handleRouteMutation)
	}
	d.sshRouting = sshrouting.NewManager(d.paths, d.selfBinaryPath())
	d.authBroker = sensitiveauth.NewBroker(d.paths, d.helperBinaryPath(), d.logger, d)

	if len(password) > 0 {
		if _, err := os.Stat(d.paths.VaultFile()); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("checking vault file: %w", err)
			}
			if err := d.hydrateWithPassword(password); err != nil {
				return err
			}
		}
		if _, err := d.authBroker.AuthorizeWithPassword(context.Background(), sensitiveauth.ActionView, password); err != nil {
			return err
		}
	}

	d.activityLog = activity.NewActivityLog(1000)
	d.sessionMu.Lock()
	d.initSyncLocked()
	err := d.startIPC()
	if err == nil {
		err = d.startAgentLocked()
	}
	d.sessionMu.Unlock()
	if err != nil {
		return err
	}

	if err := d.refreshSSHRouting(); err != nil {
		return err
	}

	if err := d.writePID(); err != nil {
		return err
	}

	d.logger.Info("daemon ready",
		"keys", d.activeKeyCount(),
		"active_session", d.HasActiveSession(),
		"agent_socket", d.paths.AgentSocket(),
		"ctl_socket", d.paths.CtlSocket(),
	)

	d.waitForSignal()
	return nil
}

func (d *Daemon) KeyStore() *vault.KeyStore {
	return d.keyStore
}

func (d *Daemon) Stop() {
	d.stopOnce.Do(func() {
		close(d.stop)
	})
}

func (d *Daemon) helperBinaryPath() string {
	return filepath.Join(filepath.Dir(d.selfBinaryPath()), helperBinaryName())
}

func (d *Daemon) selfBinaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func helperBinaryName() string {
	name := "forged-auth"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func (d *Daemon) refreshSSHRouting() error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	return d.refreshSSHRoutingLocked()
}

func (d *Daemon) refreshSSHRoutingLocked() error {
	if d.sshRouting == nil {
		return nil
	}
	var keys []vault.Key
	if d.keyStore != nil {
		keys = d.keyStore.List()
	}
	if err := d.sshRouting.Refresh(keys); err != nil {
		d.logger.Warn("refreshing ssh routing failed", "error", err)
		return fmt.Errorf("Refreshing SSH routing: %w", err)
	}
	return nil
}

func (d *Daemon) setupLogging() error {
	logPath := d.paths.LogFile()
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return err
	}

	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
	}

	// Don't MultiWriter to os.Stderr: when running under launchd/systemd the
	// service config already redirects stderr into this same log file, which
	// would double every line.
	d.logger = slog.New(slog.NewTextHandler(lj, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return nil
}

func (d *Daemon) cleanStaleState() error {
	pidPath := d.paths.PIDFile()
	if runtime.GOOS != "windows" {
		if data, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				if platform.ProcessAlive(pid) {
					if command, inspectErr := processCommandLine(pid); inspectErr == nil {
						if isForgedDaemonCommand(command) {
							return fmt.Errorf("Daemon already running (PID %d)", pid)
						}
						if d.logger != nil {
							d.logger.Warn("ignoring stale daemon pid file because pid belongs to another process", "pid", pid, "command", command)
						}
					} else {
						return fmt.Errorf("Daemon already running (PID %d)", pid)
					}
				}
			}
			os.Remove(pidPath)
		}
	}

	for _, sock := range []string{d.paths.AgentSocket(), d.paths.CtlSocket()} {
		if err := platform.CleanStaleSocket(sock); err != nil {
			return fmt.Errorf("Socket %s: %w", sock, err)
		}
	}

	if runtime.GOOS == "windows" {
		// PID reuse cannot be disambiguated without command-line inspection.
		// Recheck the authoritative named pipes before removing stale metadata.
		for _, pipe := range []string{d.paths.AgentSocket(), d.paths.CtlSocket()} {
			if platform.IsSocketAlive(pipe) {
				return fmt.Errorf("Daemon already running")
			}
		}
		os.Remove(pidPath)
		return nil
	}

	return nil
}

func (d *Daemon) writePID() error {
	pidPath := d.paths.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		return fmt.Errorf("Creating PID directory: %w", err)
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

func (d *Daemon) startIPC() error {
	ctlPath := d.paths.CtlSocket()
	if err := ensureSocketDirectory(ctlPath); err != nil {
		return err
	}

	d.ipcServer = ipc.NewServer(ctlPath, d.vault, d.keyStore, d.activityLog, d.logger)
	d.ipcServer.SetSyncLinkHandler(d.handleSyncLink)
	d.ipcServer.SetSyncUnlinkHandler(d.handleSyncUnlink)
	d.ipcServer.SetAccountReplaceHandler(d.handleAccountReplace)
	d.ipcServer.SetAccountClearHandler(d.handleAccountClear)
	d.ipcServer.SetSensitiveAuthBroker(d.authBroker)
	d.ipcServer.SetSyncError(d.syncError)
	if d.syncBus != nil {
		d.ipcServer.SetSyncBus(d.syncBus)
	}
	d.ipcServer.SetOnKeyChange(func() {
		if err := d.refreshSSHRouting(); err != nil {
			d.logger.Warn("refreshing ssh routing after key change failed", "error", err)
		}
	})
	d.ipcServer.SetOnVaultChange(d.handleRouteMutation)
	d.ipcServer.SetOnReadSync(func() {
		if err := d.refreshSSHRouting(); err != nil {
			d.logger.Warn("refreshing ssh routing after sync failed", "error", err)
		}
	})
	if d.routeService != nil {
		d.ipcServer.SetSSHRouteHandler(d.routeService)
	}
	if err := d.ipcServer.Start(); err != nil {
		return fmt.Errorf("Starting IPC server: %w", err)
	}

	d.logger.Info("ipc server started", "socket", ctlPath)
	return nil
}

func (d *Daemon) startAgentLocked() error {
	agentPath := d.paths.AgentSocket()
	if err := ensureSocketDirectory(agentPath); err != nil {
		return err
	}

	d.agent = forgedagent.New(d.keyStore)
	d.agent.SetSyncCoordinator(d.syncBus)
	if d.routeService != nil {
		d.agent.SetRouteSessions(d.routeService)
	}
	d.agent.SetSensitiveAuthorizer(d.authBroker)
	d.agentServer = forgedagent.NewServer(agentPath, d.agent, d.logger)
	if err := d.agentServer.Start(); err != nil {
		return fmt.Errorf("Starting agent server: %w", err)
	}

	d.logger.Info("ssh agent started", "socket", agentPath)
	return nil
}

func ensureSocketDirectory(socketPath string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}
	return nil
}

type syncCredentials struct {
	ServerURL string `json:"server_url"`
	UserID    string `json:"user_id"`
	Token     string `json:"-"`
}

type syncCandidate struct {
	vault      *vault.Vault
	generation uint64
	serverURL  string
	userID     string
	state      *forgedsync.SyncState
	stateStore *forgedsync.StateStore
	engine     *forgedsync.Engine
}

type linkRun struct {
	candidate *syncCandidate
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
}

type syncInitRun struct {
	generation uint64
	vault      *vault.Vault
}

// syncStateSnapshot is a stable, parsed view of both account-transition
// state files. It is loaded without sessionMu, then its encrypted history is
// validated during a short live-session section before a credential commit.
type syncStateSnapshot struct {
	active syncStateFileSnapshot
	backup syncStateFileSnapshot
}

type syncStateFileSnapshot struct {
	path   string
	state  *forgedsync.SyncState
	exists bool
	hash   [sha256.Size]byte
}

type syncStateFileError struct {
	path string
	err  error
}

func (e *syncStateFileError) Error() string {
	return fmt.Sprintf("sync state %s: %v", e.path, e.err)
}

func (e *syncStateFileError) Unwrap() error {
	return e.err
}

const (
	syncLinkTimeout       = 25 * time.Second
	syncStateRecoveryHint = "Sync is paused because its local history needs recovery. Restore a verified sync-state.json backup before removing the recovery marker."
)

func (d *Daemon) initialSyncContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), syncLinkTimeout)
	go func() {
		select {
		case <-d.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (d *Daemon) initSyncLocked() {
	if d.activeAccountTransition != 0 || d.syncSuppressed || d.syncPending || d.syncDraining != nil {
		return
	}
	if d.syncBus != nil || d.vault == nil {
		return
	}
	d.syncGeneration++
	run := &syncInitRun{generation: d.syncGeneration, vault: d.vault}
	d.syncInitRun = run
	d.syncPending = true
	go d.finishSyncInit(run)
}

func (d *Daemon) finishSyncInit(run *syncInitRun) {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()

	creds, err := accountauth.Load(d.paths)

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	if err != nil || creds.ServerURL == "" || accountauth.CurrentToken(creds) == "" {
		d.clearSyncInitRunLocked(run)
		d.sessionMu.Unlock()
		return
	}
	if d.syncStateRecoveryMarkedLocked() {
		d.clearSyncInitRunLocked(run)
		d.sessionMu.Unlock()
		return
	}
	if err := d.recoverSyncStateLocked(creds); err != nil {
		d.clearSyncInitRunLocked(run)
		if errors.Is(err, forgedsync.ErrStateCorrupt) || errors.Is(err, forgedsync.ErrStateRecoveryRequired) {
			d.setSyncStateRecoveryLocked()
			d.sessionMu.Unlock()
			return
		}
		d.logger.Warn("recovering sync state transaction failed", "error", err)
		d.scheduleSyncRetryLocked(run.generation)
		d.sessionMu.Unlock()
		return
	}

	candidate, err := d.prepareSyncCandidate(syncCredentials{
		ServerURL: creds.ServerURL,
		UserID:    creds.UserID,
		Token:     accountauth.CurrentToken(creds),
	})
	if err != nil {
		d.clearSyncInitRunLocked(run)
		if d.handleSyncStateFailureLocked(forgedsync.NewStateStore(d.paths.SyncStateFile()), err) {
			d.sessionMu.Unlock()
			return
		}
		d.logger.Warn("initializing sync failed", "error", err)
		d.sessionMu.Unlock()
		return
	}
	candidate.generation = run.generation

	if creds.UserID != "" && (candidate.state.LinkedUserID != creds.UserID || (candidate.state.LastKnownServerVersion == 0 && len(candidate.state.LastSyncedBaseBlob) == 0)) {
		if err := d.markSyncDirtyLocked(); err != nil {
			d.clearSyncInitRunLocked(run)
			d.logger.Warn("initializing sync failed", "error", err)
			d.sessionMu.Unlock()
			return
		}
		d.syncInitRun = nil
		ctx, cancel := d.initialSyncContext()
		link := &linkRun{
			candidate: candidate,
			ctx:       ctx,
			cancel:    cancel,
			done:      make(chan struct{}),
		}
		d.linkRun = link
		d.sessionMu.Unlock()
		go d.finishInitialSync(link, creds.UserID)
		return
	}

	d.sessionMu.Unlock()
	err = accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
		if err := validateSyncIdentity(stored, candidate.serverURL, candidate.userID); err != nil {
			return err
		}
		return nil
	})

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	d.clearSyncInitRunLocked(run)
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		d.sessionMu.Unlock()
		return
	}
	bus, err := d.activateSyncCandidateLocked(candidate)
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		d.sessionMu.Unlock()
		return
	}
	d.sessionMu.Unlock()
	go bus.LifecycleRefresh("daemon_start")
}

func (d *Daemon) syncInitRunCurrentLocked(run *syncInitRun) bool {
	return run != nil && d.syncInitRun == run && run.generation == d.syncGeneration &&
		run.vault != nil && d.vault == run.vault && d.activeAccountTransition == 0 &&
		!d.syncSuppressed && d.syncPending && d.syncBus == nil && d.syncDraining == nil && d.linkRun == nil
}

func (d *Daemon) clearSyncInitRunLocked(run *syncInitRun) bool {
	if d.syncInitRun != run {
		return false
	}
	d.syncInitRun = nil
	d.syncPending = false
	return true
}

func (d *Daemon) cancelSyncInitRunLocked() {
	if d.syncInitRun != nil {
		d.syncInitRun = nil
	}
}

func (d *Daemon) prepareSyncCandidate(creds syncCredentials) (*syncCandidate, error) {
	if d.vault == nil {
		return nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}

	stateStore := forgedsync.NewStateStore(d.paths.SyncStateFile())
	state, err := stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("Loading sync state: %w", err)
	}
	if state != nil {
		if err := forgedsync.ValidateStateHistory(d.vault, state); err != nil {
			return nil, fmt.Errorf("validating sync state: %w", err)
		}
	}
	if state == nil {
		defaultState := forgedsync.DefaultSyncState(uuid.NewString())
		state = &defaultState
	}
	if state.DeviceID == "" {
		state.DeviceID = uuid.NewString()
	}
	hasSyncHistory := state.LinkedUserID != "" || state.LastKnownServerVersion != 0 || len(state.LastSyncedBaseBlob) != 0
	if hasSyncHistory && state.ServerURL != "" && !sameSyncServer(state.ServerURL, creds.ServerURL) {
		return nil, fmt.Errorf("Local sync state belongs to another server; unlink it before linking this account")
	}
	if state.LinkedUserID != "" && creds.UserID != "" && state.LinkedUserID != creds.UserID {
		return nil, fmt.Errorf("Local vault is linked to a different account; unlink it first")
	}
	state.ServerURL = creds.ServerURL

	client := forgedsync.NewClientWithTokenSource(creds.ServerURL, state.DeviceID, d.syncTokenSource(creds.ServerURL, state.LinkedUserID))
	if creds.Token != "" {
		client = forgedsync.NewClient(creds.ServerURL, creds.Token, state.DeviceID)
	}
	engine := forgedsync.NewEngine(d.vault, client, d.logger)
	return &syncCandidate{
		vault:      d.vault,
		serverURL:  creds.ServerURL,
		userID:     creds.UserID,
		state:      state,
		stateStore: stateStore,
		engine:     engine,
	}, nil
}

func (d *Daemon) handleSyncLink(args ipc.SyncLinkArgs) error {
	if _, err := d.requireSyncIdentity(args.ServerURL, args.UserID); err != nil {
		return err
	}

	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	run, bus, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return err
	}
	waitForLinkRun(run)
	waitForSyncBus(bus)
	_, identityErr := d.requireSyncIdentity(args.ServerURL, args.UserID)

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if identityErr != nil {
		d.finishAccountTransitionLocked(transition, true, identityErr)
		return identityErr
	}
	d.finishAccountTransitionLocked(transition, true, nil)
	d.logger.Info("sync link scheduled", "user_id", args.UserID)
	return nil
}

func (d *Daemon) handleAccountReplace(args ipc.AccountCredentialsArgs) error {
	creds := accountauth.Credentials{
		ServerURL:        args.ServerURL,
		Token:            args.Token,
		AccessToken:      args.AccessToken,
		AccessExpiresAt:  args.AccessExpiresAt,
		RefreshToken:     args.RefreshToken,
		RefreshExpiresAt: args.RefreshExpiresAt,
		UserID:           args.UserID,
		Email:            args.Email,
		Name:             args.Name,
		ChangeID:         args.ChangeID,
	}
	if err := accountauth.ValidateCredentials(creds); err != nil {
		return err
	}
	if strings.TrimSpace(creds.ChangeID) == "" {
		return fmt.Errorf("Account change ID is required")
	}

	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	run, bus, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return err
	}
	waitForLinkRun(run)
	waitForSyncBus(bus)

	snapshot, err := d.preflightSyncState()
	if err == nil {
		err = accountauth.WithCredentialsLock(d.paths, func() error {
			var verifyErr error
			snapshot, verifyErr = snapshot.reloadIfUnchanged(d.paths)
			if verifyErr != nil {
				d.quarantineSyncStateFailure(verifyErr)
				return verifyErr
			}
			if current, err := accountauth.LoadLocked(d.paths); err == nil {
				if err := snapshot.recover(current); err != nil {
					return err
				}
			}
			preserveState, err := snapshot.matchesAccount(creds)
			if err != nil {
				return err
			}
			staged := false
			if !preserveState {
				staged, err = snapshot.stage()
				if err != nil {
					return err
				}
			}
			if err := accountauth.SaveLocked(d.paths, creds); err != nil {
				if restoreErr := snapshot.restore(staged); restoreErr != nil {
					return errors.Join(err, restoreErr)
				}
				return err
			}
			if staged {
				if err := snapshot.removeBackup(); err != nil {
					d.logger.Warn("removing staged sync state after account replacement failed", "error", err)
				}
			}
			return nil
		})
	}

	d.sessionMu.Lock()
	d.finishAccountTransitionLocked(transition, true, err)
	if err != nil {
		d.sessionMu.Unlock()
		return err
	}
	d.sessionMu.Unlock()
	d.logger.Info("account replacement committed", "user_id", args.UserID)
	return nil
}

func (d *Daemon) handleAccountClear() error {
	creds, err := d.commitAccountClear()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := accountauth.RevokeRemoteSession(ctx, creds); err != nil {
		d.logger.Warn("revoking remote session after logout failed", "error", err)
	}
	d.logger.Info("account cleared")
	return nil
}

func (d *Daemon) commitAccountClear() (accountauth.Credentials, error) {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	run, bus, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return accountauth.Credentials{}, err
	}
	waitForLinkRun(run)
	waitForSyncBus(bus)

	snapshot, stateErr := d.preflightSyncState()
	var creds accountauth.Credentials
	stateRemoved := false
	err = accountauth.WithCredentialsLock(d.paths, func() error {
		creds, _ = accountauth.LoadLocked(d.paths)
		if err := accountauth.DeleteLocked(d.paths); err != nil {
			return err
		}
		if stateErr == nil {
			var verifyErr error
			snapshot, verifyErr = snapshot.reloadIfUnchanged(d.paths)
			if verifyErr != nil {
				d.quarantineSyncStateFailure(verifyErr)
				stateErr = verifyErr
			} else if removeErr := snapshot.remove(); removeErr != nil {
				stateErr = removeErr
			} else {
				stateRemoved = true
			}
		}
		if stateErr != nil {
			d.logger.Warn("removing sync state after logout failed", "error", stateErr)
		}
		if err := os.Remove(d.paths.SyncDirtyFile()); err != nil && !os.IsNotExist(err) {
			d.logger.Warn("removing sync dirty marker after logout failed", "error", err)
		}
		return nil
	})
	d.sessionMu.Lock()
	if stateRemoved {
		d.clearSyncStateRecoveryLocked()
	}
	if err != nil {
		d.finishAccountTransitionLocked(transition, true, stateErr)
		d.sessionMu.Unlock()
		return accountauth.Credentials{}, err
	}
	d.finishAccountTransitionLocked(transition, false, stateErr)
	d.sessionMu.Unlock()
	return creds, nil
}

func (d *Daemon) beginAccountChangeLocked() (*linkRun, *forgedsync.Bus, uint64, error) {
	if d.vault != nil && d.keyStore != nil {
		// Keep the dirty marker and bus state durable before detaching. A
		// stopped clean bus must not win this transition after a crash.
		if err := d.markSyncDirtyLocked(); err != nil {
			return nil, nil, 0, err
		}
		if d.syncBus != nil {
			d.syncBus.LocalMutation("account_link_transition")
		}
	}
	d.accountTransition++
	transition := d.accountTransition
	d.activeAccountTransition = transition
	d.cancelSyncInitRunLocked()
	run := d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0
	return run, d.detachSyncBusLocked(), transition, nil
}

func (d *Daemon) finishAccountTransitionLocked(transition uint64, resumeSync bool, stateErr error) bool {
	if d.activeAccountTransition != transition {
		return false
	}
	d.activeAccountTransition = 0
	d.recordSyncStateFailureLocked(stateErr)
	if resumeSync {
		d.syncSuppressed = false
		d.initSyncLocked()
	}
	return true
}

func (d *Daemon) preflightSyncState() (syncStateSnapshot, error) {
	snapshot, err := loadSyncStateSnapshot(d.paths)
	if err != nil {
		d.quarantineSyncStateFailure(err)
		return syncStateSnapshot{}, err
	}

	d.sessionMu.Lock()
	err = d.validateSyncStateSnapshotLocked(snapshot)
	d.sessionMu.Unlock()
	if err != nil {
		d.quarantineSyncStateFailure(err)
	}
	return snapshot, err
}

func (d *Daemon) validateSyncStateSnapshotLocked(snapshot syncStateSnapshot) error {
	for _, file := range []syncStateFileSnapshot{snapshot.active, snapshot.backup} {
		if file.state == nil {
			continue
		}
		if len(file.state.LastSyncedBaseBlob) > 0 && d.vault == nil {
			return &syncStateFileError{path: file.path, err: fmt.Errorf("vault is locked; unlock Forged before changing sync state")}
		}
		if err := forgedsync.ValidateStateHistory(d.vault, file.state); err != nil {
			return &syncStateFileError{path: file.path, err: err}
		}
	}
	return nil
}

func loadSyncStateSnapshot(paths config.Paths) (syncStateSnapshot, error) {
	active, err := loadSyncStateFileSnapshot(paths.SyncStateFile())
	if err != nil {
		return syncStateSnapshot{}, err
	}
	backup, err := loadSyncStateFileSnapshot(paths.SyncStateFile() + ".account-change")
	if err != nil {
		return syncStateSnapshot{}, err
	}
	return syncStateSnapshot{active: active, backup: backup}, nil
}

func loadSyncStateFileSnapshot(path string) (syncStateFileSnapshot, error) {
	for range 2 {
		before, beforeExists, err := syncStateFileHash(path)
		if err != nil {
			return syncStateFileSnapshot{}, &syncStateFileError{path: path, err: err}
		}

		state, err := forgedsync.NewStateStore(path).Load()
		if err != nil {
			return syncStateFileSnapshot{}, &syncStateFileError{path: path, err: err}
		}

		after, afterExists, err := syncStateFileHash(path)
		if err != nil {
			return syncStateFileSnapshot{}, &syncStateFileError{path: path, err: err}
		}
		if beforeExists == afterExists && (!afterExists || before == after) && (state != nil) == afterExists {
			return syncStateFileSnapshot{
				path:   path,
				state:  state,
				exists: afterExists,
				hash:   after,
			}, nil
		}
	}
	return syncStateFileSnapshot{}, &syncStateFileError{
		path: path,
		err:  fmt.Errorf("%w: sync state changed while reading", forgedsync.ErrStateRecoveryRequired),
	}
}

func syncStateFileHash(path string) ([sha256.Size]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return [sha256.Size]byte{}, false, nil
	}
	if err != nil {
		return [sha256.Size]byte{}, false, fmt.Errorf("reading sync state: %w", err)
	}
	return sha256.Sum256(data), true, nil
}

func (s syncStateSnapshot) reloadIfUnchanged(paths config.Paths) (syncStateSnapshot, error) {
	current, err := loadSyncStateSnapshot(paths)
	if err != nil {
		return syncStateSnapshot{}, err
	}
	if s.active.sameFile(current.active) && s.backup.sameFile(current.backup) {
		return current, nil
	}
	path := s.active.path
	if !s.backup.sameFile(current.backup) {
		path = s.backup.path
	}
	return syncStateSnapshot{}, &syncStateFileError{
		path: path,
		err:  fmt.Errorf("%w: sync state changed during account transition", forgedsync.ErrStateRecoveryRequired),
	}
}

func (s syncStateFileSnapshot) sameFile(other syncStateFileSnapshot) bool {
	return s.exists == other.exists && (!s.exists || s.hash == other.hash)
}

func (s *syncStateSnapshot) recover(creds accountauth.Credentials) error {
	if !s.backup.exists {
		return nil
	}
	if s.active.exists {
		return syncStateRecoveryError("active and staged sync state both exist")
	}
	if s.backup.state.LinkedUserID != creds.UserID || !sameSyncServer(s.backup.state.ServerURL, creds.ServerURL) {
		return syncStateRecoveryError("staged sync state does not match the saved account")
	}
	activePath := s.active.path
	backupPath := s.backup.path
	if err := moveSyncStateFileNoReplace(s.backup.path, s.active.path, "restoring staged sync state"); err != nil {
		return err
	}
	s.active = s.backup
	s.active.path = activePath
	s.backup = syncStateFileSnapshot{path: backupPath}
	return nil
}

func (s syncStateSnapshot) matchesAccount(creds accountauth.Credentials) (bool, error) {
	if s.backup.exists {
		return false, syncStateRecoveryError("staged sync state requires recovery")
	}
	return s.active.exists && s.active.state.LinkedUserID == creds.UserID && sameSyncServer(s.active.state.ServerURL, creds.ServerURL), nil
}

func (s *syncStateSnapshot) stage() (bool, error) {
	if s.backup.exists {
		return false, syncStateRecoveryError("staged sync state requires recovery")
	}
	if !s.active.exists {
		return false, nil
	}
	activePath := s.active.path
	backupPath := s.backup.path
	if err := moveSyncStateFileNoReplace(s.active.path, s.backup.path, "staging sync state"); err != nil {
		return false, err
	}
	s.backup = s.active
	s.backup.path = backupPath
	s.active = syncStateFileSnapshot{path: activePath}
	return true, nil
}

func (s syncStateSnapshot) restore(staged bool) error {
	if !staged {
		return nil
	}
	if _, err := os.Lstat(s.active.path); err == nil {
		return syncStateRecoveryError("active sync state appeared while restoring staged state")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading active sync state before restore: %w", err)
	}
	return moveSyncStateFileNoReplace(s.backup.path, s.active.path, "restoring sync state")
}

func (s syncStateSnapshot) remove() error {
	if s.active.exists {
		if err := removeSyncStateFile(s.active.path, "stale"); err != nil {
			return err
		}
	}
	if !s.backup.exists {
		return nil
	}
	return removeSyncStateFile(s.backup.path, "staged")
}

func (s syncStateSnapshot) removeBackup() error {
	if !s.backup.exists {
		return nil
	}
	return removeSyncStateFile(s.backup.path, "staged")
}

func syncStateRecoveryError(reason string) error {
	return fmt.Errorf("%w: %s", forgedsync.ErrStateRecoveryRequired, reason)
}

func moveSyncStateFileNoReplace(source, destination, action string) error {
	if err := forgedsync.NewStateStore(source).MoveTo(destination); err != nil {
		return fmt.Errorf("%w: %s: %v", forgedsync.ErrStateRecoveryRequired, action, err)
	}
	return nil
}

func (d *Daemon) syncStateBackupPath() string {
	return d.paths.SyncStateFile() + ".account-change"
}

func (d *Daemon) recoverSyncStateLocked(creds accountauth.Credentials) error {
	active, err := d.loadSyncStateLocked(forgedsync.NewStateStore(d.paths.SyncStateFile()))
	if err != nil {
		return err
	}
	backupPath := d.syncStateBackupPath()
	backup, err := d.loadSyncStateLocked(forgedsync.NewStateStore(backupPath))
	if err != nil {
		return err
	}
	if backup == nil {
		return nil
	}
	if active != nil {
		return d.syncStateRecoveryRequiredLocked("active and staged sync state both exist")
	}
	if backup.LinkedUserID != creds.UserID || !sameSyncServer(backup.ServerURL, creds.ServerURL) {
		return d.syncStateRecoveryRequiredLocked("staged sync state does not match the saved account")
	}
	if err := d.moveSyncStateNoReplace(backupPath, d.paths.SyncStateFile(), "restoring staged sync state"); err != nil {
		return err
	}
	return nil
}

func removeSyncStateFile(path, label string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s sync state: %w", label, err)
	}
	return nil
}

func (d *Daemon) moveSyncStateNoReplace(source, destination, action string) error {
	if err := forgedsync.NewStateStore(source).MoveTo(destination); err != nil {
		d.setSyncStateRecoveryLocked()
		return fmt.Errorf("%w: %s: %v", forgedsync.ErrStateRecoveryRequired, action, err)
	}
	return nil
}

func (d *Daemon) loadSyncStateLocked(store *forgedsync.StateStore) (*forgedsync.SyncState, error) {
	state, err := store.Load()
	if err != nil {
		d.handleSyncStateFailureLocked(store, err)
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	if len(state.LastSyncedBaseBlob) > 0 && d.vault == nil {
		return nil, fmt.Errorf("vault is locked; unlock Forged before changing sync state")
	}
	if err := forgedsync.ValidateStateHistory(d.vault, state); err != nil {
		d.handleSyncStateFailureLocked(store, err)
		return nil, err
	}
	return state, nil
}

func (d *Daemon) handleSyncStateFailureLocked(store *forgedsync.StateStore, err error) bool {
	if !handleSyncStateFailure(store, d.logger, err) {
		return false
	}
	d.setSyncStateRecoveryLocked()
	return true
}

func handleSyncStateFailure(store *forgedsync.StateStore, logger *slog.Logger, err error) bool {
	if !errors.Is(err, forgedsync.ErrStateCorrupt) && !errors.Is(err, forgedsync.ErrStateRecoveryRequired) {
		return false
	}
	if errors.Is(err, forgedsync.ErrStateCorrupt) {
		recoveryRequired, markerErr := store.RecoveryRequired()
		if markerErr != nil {
			if logger != nil {
				logger.Warn("reading sync recovery marker failed", "error", markerErr)
			}
		} else if !recoveryRequired {
			if quarantineErr := store.Quarantine(); quarantineErr != nil && logger != nil {
				logger.Warn("quarantining corrupt sync state failed", "error", quarantineErr)
			}
		}
	}
	return true
}

func (d *Daemon) quarantineSyncStateFailure(err error) {
	var fileErr *syncStateFileError
	if errors.As(err, &fileErr) {
		handleSyncStateFailure(forgedsync.NewStateStore(fileErr.path), d.logger, fileErr.err)
	}
}

func (d *Daemon) recordSyncStateFailureLocked(err error) {
	if errors.Is(err, forgedsync.ErrStateCorrupt) || errors.Is(err, forgedsync.ErrStateRecoveryRequired) {
		d.setSyncStateRecoveryLocked()
	}
}

func (d *Daemon) syncStateRecoveryRequiredLocked(reason string) error {
	d.setSyncStateRecoveryLocked()
	return fmt.Errorf("%w: %s", forgedsync.ErrStateRecoveryRequired, reason)
}

func (d *Daemon) setSyncStateRecoveryLocked() {
	d.syncError = syncStateRecoveryHint
	if d.ipcServer != nil {
		d.ipcServer.SetSyncError(d.syncError)
	}
}

func (d *Daemon) clearSyncStateRecoveryLocked() {
	d.syncError = ""
	if d.ipcServer != nil {
		d.ipcServer.SetSyncError("")
	}
}

func (d *Daemon) syncStateRecoveryMarkedLocked() bool {
	for _, path := range []string{d.paths.SyncStateFile(), d.syncStateBackupPath()} {
		recoveryRequired, err := forgedsync.NewStateStore(path).RecoveryRequired()
		if err != nil {
			if d.logger != nil {
				d.logger.Warn("reading sync recovery marker failed", "error", err)
			}
			continue
		}
		if recoveryRequired {
			d.setSyncStateRecoveryLocked()
			return true
		}
	}
	return false
}

func (d *Daemon) handleSyncUnlink() error {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	d.cancelSyncInitRunLocked()
	run := d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0
	bus := d.detachSyncBusLocked()
	d.sessionMu.Unlock()
	waitForLinkRun(run)
	waitForSyncBus(bus)

	snapshot, err := d.preflightSyncState()
	stateRemoved := false
	if err == nil {
		snapshot, err = snapshot.reloadIfUnchanged(d.paths)
	}
	if err == nil {
		err = snapshot.remove()
		stateRemoved = err == nil
	}
	if err != nil {
		d.quarantineSyncStateFailure(err)
	}
	if err == nil {
		if removeErr := os.Remove(d.paths.SyncDirtyFile()); removeErr != nil && !os.IsNotExist(removeErr) {
			err = fmt.Errorf("Removing sync dirty flag: %w", removeErr)
		}
	}

	d.sessionMu.Lock()
	d.recordSyncStateFailureLocked(err)
	if stateRemoved {
		d.clearSyncStateRecoveryLocked()
	}
	d.sessionMu.Unlock()
	if err != nil {
		return err
	}

	d.logger.Info("sync unlinked")
	return nil
}

func (d *Daemon) finishInitialSync(run *linkRun, userID string) {
	defer close(run.done)
	defer run.cancel()

	d.sessionMu.Lock()
	if !d.linkRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	d.sessionMu.Unlock()

	candidate := run.candidate
	refreshed, err := accountauth.EnsureFresh(run.ctx, d.paths)
	if err == nil {
		err = validateSyncIdentity(refreshed, candidate.serverURL, userID)
	}
	if err == nil {
		err = accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
			if err := validateSyncIdentity(stored, candidate.serverURL, userID); err != nil {
				return err
			}
			client := forgedsync.NewClient(candidate.serverURL, accountauth.CurrentToken(stored), candidate.state.DeviceID)
			candidate.engine = forgedsync.NewEngine(candidate.vault, client, d.logger)
			return candidate.engine.ReconcileOnLink(run.ctx, candidate.state, userID, candidate.serverURL)
		})
	}

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if !d.linkRunCurrentLocked(run) {
		return
	}
	d.linkRun = nil
	d.syncPending = false
	if err != nil {
		d.logger.Warn("link reconcile failed", "error", err)
		d.scheduleSyncRetryLocked(candidate.generation)
		return
	}
	bus, err := d.activateSyncCandidateLocked(candidate)
	if err != nil {
		d.logger.Warn("link reconcile failed", "error", err)
		d.scheduleSyncRetryLocked(candidate.generation)
		return
	}
	if bus != nil {
		d.syncRetryDelay = 0
		go bus.LifecycleRefresh("link_complete")
	}
}

func (d *Daemon) linkRunCurrentLocked(run *linkRun) bool {
	if run == nil || d.linkRun != run || run.candidate == nil {
		return false
	}
	candidate := run.candidate
	return candidate.generation == d.syncGeneration &&
		!d.syncSuppressed &&
		d.vault != nil && d.vault == candidate.vault &&
		d.syncBus == nil
}

func (d *Daemon) cancelLinkRunLocked() *linkRun {
	run := d.linkRun
	if run == nil {
		return nil
	}
	d.linkRun = nil
	run.cancel()
	return run
}

func waitForLinkRun(run *linkRun) {
	if run != nil {
		<-run.done
	}
}

func (d *Daemon) scheduleSyncRetryLocked(generation uint64) {
	if generation != d.syncGeneration || d.syncSuppressed || d.vault == nil {
		return
	}
	delay := d.syncRetryDelay
	if delay == 0 {
		delay = 5 * time.Second
	} else {
		delay = min(delay*2, 5*time.Minute)
	}
	d.syncRetryDelay = delay
	d.syncPending = true
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-d.stop:
			return
		}
		d.sessionMu.Lock()
		defer d.sessionMu.Unlock()
		if generation != d.syncGeneration || d.syncSuppressed || d.syncBus != nil {
			return
		}
		d.syncPending = false
		d.initSyncLocked()
	}()
}

func (d *Daemon) activateSyncCandidateLocked(candidate *syncCandidate) (*forgedsync.Bus, error) {
	if candidate.generation != d.syncGeneration || d.syncSuppressed {
		return nil, fmt.Errorf("Sync link was superseded")
	}
	if d.vault == nil || d.vault != candidate.vault {
		return nil, fmt.Errorf("Vault session changed while linking sync")
	}
	if d.syncDraining != nil {
		return nil, fmt.Errorf("previous sync work is still stopping")
	}
	if _, err := os.Stat(d.paths.SyncDirtyFile()); err == nil {
		candidate.state.MarkDirty("", time.Time{})
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Reading sync dirty marker: %w", err)
	}
	if err := candidate.stateStore.Save(candidate.state); err != nil {
		return nil, fmt.Errorf("Saving sync state: %w", err)
	}

	client := forgedsync.NewClientWithTokenSource(candidate.serverURL, candidate.state.DeviceID, d.syncTokenSource(candidate.serverURL, candidate.state.LinkedUserID))
	engine := forgedsync.NewEngine(candidate.vault, client, d.logger)
	bus := forgedsync.NewBus(engine, candidate.state, d.logger, forgedsync.BusConfig{
		DirtyFlagPath: d.paths.SyncDirtyFile(),
		StateStore:    candidate.stateStore,
	})
	d.clearSyncStateRecoveryLocked()
	if err := d.replaceSyncBusLocked(bus); err != nil {
		return nil, err
	}
	d.logger.Info("sync initialized", "server", candidate.serverURL, "device_id", candidate.state.DeviceID)
	return bus, nil
}

func (d *Daemon) replaceSyncBusLocked(next *forgedsync.Bus) error {
	if next == nil {
		d.detachSyncBusLocked()
		return nil
	}
	if d.syncBus != nil && d.syncBus != next {
		return fmt.Errorf("sync bus replacement requires the previous bus to stop")
	}
	if d.syncDraining != nil {
		return fmt.Errorf("previous sync work is still stopping")
	}
	d.syncBus = next
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(next)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(next)
	}
	return nil
}

func (d *Daemon) detachSyncBusLocked() *forgedsync.Bus {
	if d.syncBus == nil {
		return d.syncDraining
	}

	previous := d.syncBus
	d.syncBus = nil
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(nil)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(nil)
	}
	previous.BeginStop()
	d.syncDraining = previous
	go d.drainSyncBus(previous)
	return previous
}

func (d *Daemon) drainSyncBus(bus *forgedsync.Bus) {
	bus.Wait()

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if d.syncDraining != bus {
		return
	}
	d.syncDraining = nil
	d.initSyncLocked()
}

func waitForSyncBus(bus *forgedsync.Bus) {
	if bus != nil {
		bus.Wait()
	}
}

func (d *Daemon) waitForSyncDrain() {
	d.sessionMu.Lock()
	bus := d.syncDraining
	d.sessionMu.Unlock()
	waitForSyncBus(bus)
}

func (d *Daemon) syncTokenSource(serverURL, userID string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		creds, err := accountauth.EnsureFresh(ctx, d.paths)
		if err != nil {
			return "", fmt.Errorf("Refreshing account credentials: %w", err)
		}
		if !sameSyncServer(creds.ServerURL, serverURL) || (userID != "" && creds.UserID != userID) {
			return "", fmt.Errorf("Saved account changed while sync was active")
		}
		return accountauth.CurrentToken(creds), nil
	}
}

func (d *Daemon) requireSyncIdentity(serverURL, userID string) (accountauth.Credentials, error) {
	creds, err := accountauth.Load(d.paths)
	if err != nil {
		return accountauth.Credentials{}, fmt.Errorf("Loading linked account credentials: %w", err)
	}
	if err := validateSyncIdentity(creds, serverURL, userID); err != nil {
		return accountauth.Credentials{}, err
	}
	return creds, nil
}

func validateSyncIdentity(creds accountauth.Credentials, serverURL, userID string) error {
	if !sameSyncServer(creds.ServerURL, serverURL) || (userID != "" && creds.UserID != userID) {
		return fmt.Errorf("Saved account changed while sync was linking")
	}
	return nil
}

func sameSyncServer(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

func (d *Daemon) HasActiveSession() bool {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	return d.vault != nil && d.keyStore != nil
}

func (d *Daemon) HydrateFromEnrollment() error {
	symmetricKey, err := sensitiveauth.RecoverEnrolledSymmetricKey(d.paths)
	if err != nil {
		return err
	}
	defer zeroSecret(symmetricKey)
	if err := d.hydrateWithSymmetricKey(symmetricKey, "local_enrollment"); err != nil {
		return err
	}
	sensitiveauth.RenewLocalEnrollmentUsage(d.paths)
	return nil
}

func (d *Daemon) HydrateFromPassword(password []byte) error {
	return d.hydrateWithPassword(password)
}

func (d *Daemon) RefreshLocalEnrollment() {
	d.sessionMu.Lock()
	if d.vault == nil || d.keyStore == nil {
		d.sessionMu.Unlock()
		return
	}
	symmetricKey := d.vault.Key()
	d.sessionMu.Unlock()
	defer zeroSecret(symmetricKey)

	result, err := sensitiveauth.RefreshLocalEnrollment(d.paths, symmetricKey)
	if err != nil {
		if d.logger != nil {
			d.logger.Warn("local unlock enrollment not refreshed", "error", err)
		}
		return
	}
	if !result.Refreshed && d.logger != nil && result.Reason != "" {
		d.logger.Warn("local unlock enrollment not refreshed", "capability", result.Capability, "reason", result.Reason)
	}
}

func (d *Daemon) ClearActiveSession(reason string) {
	d.clearActiveSession(reason)
}

func (d *Daemon) hydrateWithPassword(password []byte) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	if d.vault != nil && d.keyStore != nil {
		return nil
	}

	vaultPath := d.paths.VaultFile()

	var (
		v   *vault.Vault
		err error
	)
	if _, statErr := os.Stat(vaultPath); os.IsNotExist(statErr) {
		d.logger.Info("creating new vault", "path", vaultPath)
		v, err = vault.Create(vaultPath, password)
	} else {
		d.logger.Info("opening vault", "path", vaultPath)
		v, err = vault.Open(vaultPath, password)
	}
	if err != nil {
		return fmt.Errorf("Vault: %w", err)
	}
	return d.activateVaultLocked(v, "master_password")
}

func (d *Daemon) hydrateWithSymmetricKey(symmetricKey []byte, source string) error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	if d.vault != nil && d.keyStore != nil {
		return nil
	}

	v, err := vault.OpenWithSymmetricKey(d.paths.VaultFile(), symmetricKey)
	if err != nil {
		return fmt.Errorf("Vault: %w", err)
	}
	return d.activateVaultLocked(v, source)
}

func (d *Daemon) activateVaultLocked(v *vault.Vault, source string) error {
	keyStore := vault.NewKeyStore(v)

	d.vault = v
	d.keyStore = keyStore
	if d.ipcServer != nil {
		d.initSyncLocked()
	}

	if d.routeService != nil {
		d.routeService.SetKeyStore(keyStore)
	}
	if d.ipcServer != nil {
		d.ipcServer.SetVaultState(v, keyStore)
	}
	if d.agent != nil {
		d.agent.SetKeyStore(keyStore)
	}

	if err := d.refreshSSHRoutingLocked(); err != nil && d.logger != nil {
		d.logger.Warn("refreshing ssh routing after hydrate failed", "error", err, "source", source)
	}
	if d.logger != nil {
		d.logger.Info("vault session hydrated", "source", source, "keys", len(keyStore.List()))
	}
	return nil
}

func (d *Daemon) clearActiveSession(reason string) {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	d.cancelSyncInitRunLocked()
	d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncRetryDelay = 0
	d.detachSyncBusLocked()

	if d.routeService != nil {
		d.routeService.SetKeyStore(nil)
	}
	if d.ipcServer != nil {
		d.ipcServer.SetVaultState(nil, nil)
	}
	if d.agent != nil {
		d.agent.SetKeyStore(nil)
	}

	if d.vault != nil {
		d.vault.Close()
		d.vault = nil
		d.keyStore = nil
	}

	if d.logger != nil && strings.TrimSpace(reason) != "" {
		d.logger.Info("vault session cleared", "reason", reason)
	}
}

func (d *Daemon) handleRouteMutation(reason string) {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	if d.syncBus != nil {
		d.syncBus.LocalMutation(reason)
		return
	}
	if d.vault == nil {
		return
	}
	if err := d.markSyncDirtyLocked(); err != nil {
		d.logger.Warn("persisting sync dirty marker failed", "error", err)
	}
}

func (d *Daemon) markSyncDirtyLocked() error {
	if err := os.MkdirAll(filepath.Dir(d.paths.SyncDirtyFile()), 0o700); err != nil {
		return fmt.Errorf("Creating sync dirty directory: %w", err)
	}
	if err := os.WriteFile(d.paths.SyncDirtyFile(), []byte("1"), 0o600); err != nil {
		return fmt.Errorf("Writing sync dirty marker: %w", err)
	}
	return nil
}

func (d *Daemon) activeKeyCount() int {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if d.keyStore == nil {
		return 0
	}
	return len(d.keyStore.List())
}

func (d *Daemon) waitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		d.logger.Info("received signal", "signal", s)
	case <-d.stop:
		d.logger.Info("stop requested")
	}
}

func (d *Daemon) shutdown() {
	d.shutdownOnce.Do(d.shutdownNow)
}

func (d *Daemon) shutdownNow() {
	d.logger.Info("shutting down")
	d.Stop()

	if d.agentServer != nil {
		d.agentServer.BeginStop()
	}
	if d.ipcServer != nil {
		d.ipcServer.BeginStop()
	}
	if d.authBroker != nil {
		d.authBroker.BeginStop()
	}

	if d.agentServer != nil {
		d.agentServer.Wait()
	}
	if d.ipcServer != nil {
		d.ipcServer.Wait()
	}

	if d.authBroker != nil {
		d.authBroker.Close()
	} else {
		d.clearActiveSession("shutdown")
	}
	d.waitForSyncDrain()

	removeOwnedPIDFile(d.paths.PIDFile(), os.Getpid())

	d.logger.Info("daemon stopped")
}

func zeroSecret(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func removeOwnedPIDFile(path string, pid int) {
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(pid) {
		return
	}
	os.Remove(path)
}

func IsRunning(paths config.Paths) (int, bool) {
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	if !platform.ProcessAlive(pid) {
		return 0, false
	}
	if command, err := processCommandLine(pid); err == nil && !isForgedDaemonCommand(command) {
		return 0, false
	}
	return pid, true
}

func processCommandLine(pid int) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("Process inspection unavailable")
	}
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func isForgedDaemonCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	executable := filepath.Base(fields[0])
	if !strings.HasPrefix(executable, "forged") {
		return false
	}
	for _, field := range fields[1:] {
		if field == "daemon" {
			return true
		}
	}
	return false
}
