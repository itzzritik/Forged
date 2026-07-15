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
	syncApplyGate           *syncApplyGate
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
	defer zeroSecret(password)
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
	if d.routeService != nil {
		d.routeService.SetSessionChecker(d.authBroker)
	}

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
		zeroSecret(password)
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

	d.agent = forgedagent.New(d.keyStore)
	d.agent.SetSyncCoordinator(d.syncBus)
	d.agent.SetActivityLog(d.activityLog)
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
	applyGate *syncApplyGate
}

// syncApplyGate serializes a bounded local state operation. Revocation waits
// for an admitted operation, then prevents later operations without holding
// sessionMu.
type syncApplyGate struct {
	mu      sync.Mutex
	revoked bool
}

func (g *syncApplyGate) begin() bool {
	g.mu.Lock()
	if g.revoked {
		g.mu.Unlock()
		return false
	}
	return true
}

func (g *syncApplyGate) end() {
	g.mu.Unlock()
}

func (g *syncApplyGate) revoke() {
	g.mu.Lock()
	g.revoked = true
	g.mu.Unlock()
}

type syncInitRun struct {
	generation uint64
	vault      *vault.Vault
	stateGate  *syncApplyGate
	ctx        context.Context
	cancel     context.CancelFunc
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

func (d *Daemon) expireInitialLink(run *linkRun) {
	select {
	case <-d.stop:
	case <-run.ctx.Done():
	}

	run.cancel()
	run.applyGate.revoke()
}

func (d *Daemon) initSyncLocked() {
	if d.activeAccountTransition != 0 || d.syncSuppressed || d.syncPending || d.syncDraining != nil {
		return
	}
	if d.syncBus != nil || d.vault == nil {
		return
	}
	d.syncGeneration++
	ctx, cancel := context.WithCancel(context.Background())
	run := &syncInitRun{generation: d.syncGeneration, vault: d.vault, stateGate: &syncApplyGate{}, ctx: ctx, cancel: cancel}
	d.syncInitRun = run
	d.syncPending = true
	go d.finishSyncInit(run)
}

func (d *Daemon) finishSyncInit(run *syncInitRun) {
	promoted := false
	defer func() {
		if !promoted && run.cancel != nil {
			run.cancel()
		}
	}()

	// Credential-store calls can block in OS/keychain code. Keep them outside
	// syncTransitionMu; both transition-held phases revalidate this run.
	creds, credentialErr := accountauth.Load(d.paths)
	candidate, initialLink := d.prepareSyncInit(run, creds, credentialErr)
	if candidate == nil {
		return
	}
	if initialLink {
		linkCtx, linkCancel := context.WithTimeout(run.ctx, syncLinkTimeout)
		token, err := d.prepareInitialLinkCredentials(linkCtx, candidate)
		promoted = d.finishInitialLink(run, candidate, linkCtx, linkCancel, token, err)
		if !promoted {
			linkCancel()
		}
		return
	}

	err := accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
		if err := validateSyncIdentity(stored, candidate.serverURL, candidate.userID); err != nil {
			return err
		}
		return nil
	})
	d.finishDirectSyncInit(run, candidate, err)
}

func (d *Daemon) prepareSyncInit(run *syncInitRun, creds accountauth.Credentials, credentialErr error) (*syncCandidate, bool) {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return nil, false
	}
	if credentialErr != nil || creds.ServerURL == "" || accountauth.CurrentToken(creds) == "" {
		d.clearSyncInitRunLocked(run)
		d.sessionMu.Unlock()
		return nil, false
	}
	d.sessionMu.Unlock()

	if !d.admitSyncInitState(run) {
		return nil, false
	}
	recoveryMarked := d.syncStateRecoveryMarked()
	var recoveryErr error
	var candidate *syncCandidate
	var candidateErr error
	if !recoveryMarked {
		recoveryErr = d.recoverSyncState(run.vault, creds)
		if recoveryErr == nil {
			candidate, candidateErr = d.prepareSyncCandidate(run.vault, syncCredentials{
				ServerURL: creds.ServerURL,
				UserID:    creds.UserID,
				Token:     accountauth.CurrentToken(creds),
			})
		}
	}
	run.stateGate.end()

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return nil, false
	}
	if recoveryMarked {
		d.clearSyncInitRunLocked(run)
		d.setSyncStateRecoveryLocked()
		d.sessionMu.Unlock()
		return nil, false
	}
	if recoveryErr != nil {
		d.clearSyncInitRunLocked(run)
		if errors.Is(recoveryErr, forgedsync.ErrStateCorrupt) || errors.Is(recoveryErr, forgedsync.ErrStateRecoveryRequired) {
			d.setSyncStateRecoveryLocked()
			d.sessionMu.Unlock()
			return nil, false
		}
		d.logger.Warn("recovering sync state transaction failed", "error", recoveryErr)
		d.scheduleSyncRetryLocked(run.generation)
		d.sessionMu.Unlock()
		return nil, false
	}
	if candidateErr != nil {
		d.clearSyncInitRunLocked(run)
		if errors.Is(candidateErr, forgedsync.ErrStateCorrupt) || errors.Is(candidateErr, forgedsync.ErrStateRecoveryRequired) {
			d.setSyncStateRecoveryLocked()
			d.sessionMu.Unlock()
			return nil, false
		}
		d.logger.Warn("initializing sync failed", "error", candidateErr)
		d.sessionMu.Unlock()
		return nil, false
	}
	candidate.generation = run.generation

	if creds.UserID != "" && (candidate.state.LinkedUserID != creds.UserID || (candidate.state.LastKnownServerVersion == 0 && len(candidate.state.LastSyncedBaseBlob) == 0)) {
		if err := d.markSyncDirtyLocked(); err != nil {
			d.clearSyncInitRunLocked(run)
			d.logger.Warn("initializing sync failed", "error", err)
			d.sessionMu.Unlock()
			return nil, false
		}
		d.sessionMu.Unlock()
		return candidate, true
	}

	d.sessionMu.Unlock()
	return candidate, false
}

func (d *Daemon) prepareInitialLinkCredentials(ctx context.Context, candidate *syncCandidate) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	refreshed, err := accountauth.EnsureFresh(ctx, d.paths)
	if err != nil {
		return "", err
	}
	if err := validateSyncIdentity(refreshed, candidate.serverURL, candidate.userID); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var token string
	err = accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validateSyncIdentity(stored, candidate.serverURL, candidate.userID); err != nil {
			return err
		}
		token = accountauth.CurrentToken(stored)
		if token == "" {
			return fmt.Errorf("Sync credentials are missing")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (d *Daemon) finishInitialLink(run *syncInitRun, candidate *syncCandidate, ctx context.Context, cancel context.CancelFunc, token string, authErr error) bool {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return false
	}
	if authErr == nil {
		authErr = ctx.Err()
	}
	if authErr != nil {
		d.clearSyncInitRunLocked(run)
		d.logger.Warn("link reconcile failed", "error", authErr)
		d.scheduleSyncRetryLocked(candidate.generation)
		d.sessionMu.Unlock()
		return false
	}

	linkCancel := func() {
		cancel()
		if run.cancel != nil {
			run.cancel()
		}
	}
	link := &linkRun{
		candidate: candidate,
		ctx:       ctx,
		cancel:    linkCancel,
		done:      make(chan struct{}),
		applyGate: &syncApplyGate{},
	}
	client := forgedsync.NewClient(candidate.serverURL, token, candidate.state.DeviceID)
	candidate.engine = forgedsync.NewEngineWithVaultApply(candidate.vault, client, d.logger, func(applyCtx context.Context, update func(*vault.VaultData) error) error {
		return d.applyInitialLinkUpdate(link, applyCtx, update)
	})
	d.syncInitRun = nil
	d.linkRun = link
	d.sessionMu.Unlock()
	go d.expireInitialLink(link)
	go d.finishInitialSync(link, candidate.userID)
	return true
}

func (d *Daemon) finishDirectSyncInit(run *syncInitRun, candidate *syncCandidate, identityErr error) {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()

	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	if identityErr != nil {
		d.clearSyncInitRunLocked(run)
		d.logger.Warn("initializing sync failed", "error", identityErr)
		d.sessionMu.Unlock()
		return
	}
	d.sessionMu.Unlock()

	err := d.persistSyncInitCandidate(run, candidate)

	var bus *forgedsync.Bus
	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	if err != nil {
		d.clearSyncInitRunLocked(run)
		d.logger.Warn("initializing sync failed", "error", err)
		d.sessionMu.Unlock()
		return
	}
	if run.stateGate == nil || !run.stateGate.begin() {
		d.clearSyncInitRunLocked(run)
		d.sessionMu.Unlock()
		return
	}
	if err = d.markCandidateDirtyFromFlag(candidate); err == nil {
		bus, err = d.installSyncCandidateLocked(candidate)
	}
	run.stateGate.end()
	d.clearSyncInitRunLocked(run)
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		d.sessionMu.Unlock()
		return
	}
	d.sessionMu.Unlock()
	if bus != nil {
		go bus.LifecycleRefresh("daemon_start")
	}
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
	if run.cancel != nil {
		run.cancel()
	}
	return true
}

func (d *Daemon) cancelSyncInitRunLocked() *syncInitRun {
	run := d.syncInitRun
	d.syncInitRun = nil
	if run != nil && run.cancel != nil {
		run.cancel()
	}
	return run
}

func (d *Daemon) admitSyncInitState(run *syncInitRun) bool {
	d.sessionMu.Lock()
	if !d.syncInitRunCurrentLocked(run) || run.stateGate == nil || !run.stateGate.begin() {
		d.sessionMu.Unlock()
		return false
	}
	d.sessionMu.Unlock()
	return true
}

func (d *Daemon) persistSyncInitCandidate(run *syncInitRun, candidate *syncCandidate) error {
	if !d.admitSyncInitState(run) {
		return context.Canceled
	}
	defer run.stateGate.end()
	return d.persistSyncCandidate(candidate)
}

func (d *Daemon) persistInitialLinkCandidate(run *linkRun) error {
	d.sessionMu.Lock()
	if !d.linkRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := run.ctx.Err(); err != nil {
		d.sessionMu.Unlock()
		return err
	}
	if !run.applyGate.begin() {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := run.ctx.Err(); err != nil {
		run.applyGate.end()
		d.sessionMu.Unlock()
		return err
	}
	d.sessionMu.Unlock()
	defer run.applyGate.end()
	return d.persistSyncCandidate(run.candidate)
}

func (d *Daemon) prepareSyncCandidate(v *vault.Vault, creds syncCredentials) (*syncCandidate, error) {
	if v == nil {
		return nil, fmt.Errorf("Vault is locked; open Forged to unlock")
	}

	stateStore := forgedsync.NewStateStore(d.paths.SyncStateFile())
	state, err := stateStore.Load()
	if err != nil {
		handleSyncStateFailure(stateStore, d.logger, err)
		return nil, fmt.Errorf("Loading sync state: %w", err)
	}
	if state != nil {
		if err := forgedsync.ValidateStateHistory(v, state); err != nil {
			handleSyncStateFailure(stateStore, d.logger, err)
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
	engine := forgedsync.NewEngine(v, client, d.logger)
	return &syncCandidate{
		vault:      v,
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
	run, bus, applyGate, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return err
	}
	revokeSyncApplies(run, applyGate)
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

func (d *Daemon) handleAccountReplace(args ipc.AccountCredentialsArgs) (*ipc.AccountChangeResult, error) {
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
		return nil, err
	}
	if strings.TrimSpace(creds.ChangeID) == "" {
		return nil, fmt.Errorf("Account change ID is required")
	}

	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	run, bus, applyGate, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return nil, err
	}
	revokeSyncApplies(run, applyGate)
	waitForLinkRun(run)
	waitForSyncBus(bus)

	snapshot, err := d.preflightSyncState()
	var cleanupErr error
	credentialSecretCleanupPending := false
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
			if saveErr := accountauth.SaveLocked(d.paths, creds); saveErr != nil {
				if errors.Is(saveErr, accountauth.ErrCredentialSecretCleanupPending) {
					credentialSecretCleanupPending = true
				} else {
					if restoreErr := snapshot.restore(staged); restoreErr != nil {
						return errors.Join(saveErr, restoreErr)
					}
					return saveErr
				}
			}
			if staged {
				if removeErr := snapshot.removeBackup(); removeErr != nil {
					cleanupErr = fmt.Errorf("%w: removing staged sync state after account replacement: %v", forgedsync.ErrStateRecoveryRequired, removeErr)
					d.logger.Warn("removing staged sync state after account replacement failed", "error", removeErr)
				}
			}
			return nil
		})
	}

	d.sessionMu.Lock()
	transitionErr := err
	if transitionErr == nil {
		transitionErr = cleanupErr
	}
	d.finishAccountTransitionLocked(transition, true, transitionErr)
	if err != nil {
		d.sessionMu.Unlock()
		return nil, err
	}
	d.sessionMu.Unlock()
	d.logger.Info("account replacement committed", "user_id", args.UserID)
	if cleanupErr != nil || credentialSecretCleanupPending {
		return &ipc.AccountChangeResult{
			SyncCleanupPending:             cleanupErr != nil,
			CredentialSecretCleanupPending: credentialSecretCleanupPending,
		}, nil
	}
	return nil, nil
}

func (d *Daemon) handleAccountClear() (*ipc.AccountChangeResult, error) {
	creds, syncCleanupPending, credentialSecretCleanupPending, err := d.commitAccountClear()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := accountauth.RevokeRemoteSession(ctx, creds); err != nil {
		d.logger.Warn("revoking remote session after logout failed", "error", err)
	}
	d.logger.Info("account cleared")
	if syncCleanupPending || credentialSecretCleanupPending {
		return &ipc.AccountChangeResult{
			SyncCleanupPending:             syncCleanupPending,
			CredentialSecretCleanupPending: credentialSecretCleanupPending,
		}, nil
	}
	return nil, nil
}

func (d *Daemon) commitAccountClear() (accountauth.Credentials, bool, bool, error) {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	run, bus, applyGate, transition, err := d.beginAccountChangeLocked()
	d.sessionMu.Unlock()
	if err != nil {
		return accountauth.Credentials{}, false, false, err
	}
	revokeSyncApplies(run, applyGate)
	waitForLinkRun(run)
	waitForSyncBus(bus)

	snapshot, stateErr := d.preflightSyncState()
	var creds accountauth.Credentials
	stateRemoved := false
	var dirtyErr error
	credentialSecretCleanupPending := false
	err = accountauth.WithCredentialsLock(d.paths, func() error {
		creds, _ = accountauth.LoadLocked(d.paths)
		if deleteErr := accountauth.DeleteLocked(d.paths); deleteErr != nil {
			if errors.Is(deleteErr, accountauth.ErrCredentialSecretCleanupPending) {
				credentialSecretCleanupPending = true
			} else {
				return deleteErr
			}
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
		if removeErr := os.Remove(d.paths.SyncDirtyFile()); removeErr != nil && !os.IsNotExist(removeErr) {
			dirtyErr = removeErr
			d.logger.Warn("removing sync dirty marker after logout failed", "error", removeErr)
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
		return accountauth.Credentials{}, false, false, err
	}
	transitionErr := stateErr
	if transitionErr != nil && !errors.Is(transitionErr, forgedsync.ErrStateCorrupt) && !errors.Is(transitionErr, forgedsync.ErrStateRecoveryRequired) {
		transitionErr = fmt.Errorf("%w: retaining sync state after logout: %v", forgedsync.ErrStateRecoveryRequired, transitionErr)
	}
	d.finishAccountTransitionLocked(transition, false, transitionErr)
	d.sessionMu.Unlock()
	return creds, stateErr != nil || dirtyErr != nil, credentialSecretCleanupPending, nil
}

func (d *Daemon) beginAccountChangeLocked() (*linkRun, *forgedsync.Bus, *syncApplyGate, uint64, error) {
	if d.vault != nil && d.keyStore != nil {
		// Keep the dirty marker and bus state durable before detaching. A
		// stopped clean bus must not win this transition after a crash.
		if err := d.markSyncDirtyLocked(); err != nil {
			return nil, nil, nil, 0, err
		}
		if d.syncBus != nil {
			d.syncBus.LocalMutation("account_link_transition")
		}
	}
	d.accountTransition++
	transition := d.accountTransition
	d.activeAccountTransition = transition
	_ = d.cancelSyncInitRunLocked()
	run := d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0
	bus, applyGate := d.detachSyncBusLocked()
	return run, bus, applyGate, transition, nil
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

func (d *Daemon) recoverSyncState(v *vault.Vault, creds accountauth.Credentials) error {
	active, err := d.loadSyncState(v, forgedsync.NewStateStore(d.paths.SyncStateFile()))
	if err != nil {
		return err
	}
	backupPath := d.syncStateBackupPath()
	backup, err := d.loadSyncState(v, forgedsync.NewStateStore(backupPath))
	if err != nil {
		return err
	}
	if backup == nil {
		return nil
	}
	if active != nil {
		return syncStateRecoveryError("active and staged sync state both exist")
	}
	if backup.LinkedUserID != creds.UserID || !sameSyncServer(backup.ServerURL, creds.ServerURL) {
		return syncStateRecoveryError("staged sync state does not match the saved account")
	}
	if err := moveSyncStateFileNoReplace(backupPath, d.paths.SyncStateFile(), "restoring staged sync state"); err != nil {
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

func (d *Daemon) loadSyncState(v *vault.Vault, store *forgedsync.StateStore) (*forgedsync.SyncState, error) {
	state, err := store.Load()
	if err != nil {
		handleSyncStateFailure(store, d.logger, err)
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	if len(state.LastSyncedBaseBlob) > 0 && v == nil {
		return nil, fmt.Errorf("vault is locked; unlock Forged before changing sync state")
	}
	if err := forgedsync.ValidateStateHistory(v, state); err != nil {
		handleSyncStateFailure(store, d.logger, err)
		return nil, err
	}
	return state, nil
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

func (d *Daemon) syncStateRecoveryMarked() bool {
	for _, path := range []string{d.paths.SyncStateFile(), d.syncStateBackupPath()} {
		recoveryRequired, err := forgedsync.NewStateStore(path).RecoveryRequired()
		if err != nil {
			if d.logger != nil {
				d.logger.Warn("reading sync recovery marker failed", "error", err)
			}
			continue
		}
		if recoveryRequired {
			return true
		}
	}
	return false
}

func (d *Daemon) handleSyncUnlink() error {
	d.syncTransitionMu.Lock()
	defer d.syncTransitionMu.Unlock()
	d.sessionMu.Lock()
	_ = d.cancelSyncInitRunLocked()
	run := d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0
	bus, applyGate := d.detachSyncBusLocked()
	d.sessionMu.Unlock()
	revokeSyncApplies(run, applyGate)
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
	defer func() {
		run.applyGate.revoke()
		run.cancel()
	}()

	d.sessionMu.Lock()
	if !d.linkRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	d.sessionMu.Unlock()

	candidate := run.candidate
	err := run.ctx.Err()
	if err == nil {
		err = candidate.engine.ReconcileOnLink(run.ctx, candidate.state, userID, candidate.serverURL)
	}
	if err == nil {
		err = d.persistInitialLinkCandidate(run)
	}

	d.sessionMu.Lock()
	if !d.linkRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return
	}
	if err == nil {
		if ctxErr := run.ctx.Err(); ctxErr != nil {
			err = ctxErr
		} else if !run.applyGate.begin() {
			err = context.Canceled
		} else if ctxErr := run.ctx.Err(); ctxErr != nil {
			run.applyGate.end()
			err = ctxErr
		}
	}
	var bus *forgedsync.Bus
	if err == nil {
		if err = d.markCandidateDirtyFromFlag(candidate); err == nil {
			bus, err = d.installSyncCandidateLocked(candidate)
		}
		run.applyGate.end()
	}
	d.linkRun = nil
	d.syncPending = false
	if err != nil {
		d.logger.Warn("link reconcile failed", "error", err)
		d.scheduleSyncRetryLocked(candidate.generation)
		d.sessionMu.Unlock()
		return
	}
	d.syncRetryDelay = 0
	d.sessionMu.Unlock()
	if bus != nil {
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
	return run
}

func (d *Daemon) applyInitialLinkUpdate(run *linkRun, ctx context.Context, update func(*vault.VaultData) error) error {
	d.sessionMu.Lock()
	if !d.linkRunCurrentLocked(run) {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		d.sessionMu.Unlock()
		return err
	}
	if !run.applyGate.begin() {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		run.applyGate.end()
		d.sessionMu.Unlock()
		return err
	}
	d.sessionMu.Unlock()
	defer run.applyGate.end()
	return run.candidate.vault.UpdateData(update)
}

func (d *Daemon) applyActiveSyncUpdate(candidate *syncCandidate, applyGate *syncApplyGate, ctx context.Context, update func(*vault.VaultData) error) error {
	d.sessionMu.Lock()
	if candidate == nil || candidate.generation != d.syncGeneration || candidate.vault == nil || candidate.vault != d.vault || d.syncSuppressed || d.syncBus == nil || d.syncApplyGate != applyGate {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		d.sessionMu.Unlock()
		return err
	}
	if applyGate == nil || !applyGate.begin() {
		d.sessionMu.Unlock()
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		applyGate.end()
		d.sessionMu.Unlock()
		return err
	}
	d.sessionMu.Unlock()
	defer applyGate.end()
	return candidate.vault.UpdateData(update)
}

func waitForLinkRun(run *linkRun) {
	if run != nil {
		<-run.done
	}
}

func revokeSyncApplies(run *linkRun, applyGate *syncApplyGate) {
	if run != nil {
		run.applyGate.revoke()
		run.cancel()
	}
	if applyGate != nil {
		applyGate.revoke()
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

func (d *Daemon) markCandidateDirtyFromFlag(candidate *syncCandidate) error {
	if _, err := os.Stat(d.paths.SyncDirtyFile()); err == nil {
		candidate.state.MarkDirty("", time.Time{})
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Reading sync dirty marker: %w", err)
	}
	return nil
}

func (d *Daemon) persistSyncCandidate(candidate *syncCandidate) error {
	if err := d.markCandidateDirtyFromFlag(candidate); err != nil {
		return err
	}
	if err := candidate.stateStore.Save(candidate.state); err != nil {
		return fmt.Errorf("Saving sync state: %w", err)
	}
	return nil
}

func (d *Daemon) installSyncCandidateLocked(candidate *syncCandidate) (*forgedsync.Bus, error) {
	if candidate.generation != d.syncGeneration || d.syncSuppressed {
		return nil, fmt.Errorf("Sync link was superseded")
	}
	if d.vault == nil || d.vault != candidate.vault {
		return nil, fmt.Errorf("Vault session changed while linking sync")
	}
	if d.syncDraining != nil {
		return nil, fmt.Errorf("previous sync work is still stopping")
	}

	client := forgedsync.NewClientWithTokenSource(candidate.serverURL, candidate.state.DeviceID, d.syncTokenSource(candidate.serverURL, candidate.state.LinkedUserID))
	applyGate := &syncApplyGate{}
	engine := forgedsync.NewEngineWithVaultApply(candidate.vault, client, d.logger, func(ctx context.Context, update func(*vault.VaultData) error) error {
		return d.applyActiveSyncUpdate(candidate, applyGate, ctx, update)
	})
	bus := forgedsync.NewBus(engine, candidate.state, d.logger, forgedsync.BusConfig{
		DirtyFlagPath: d.paths.SyncDirtyFile(),
		StateStore:    candidate.stateStore,
	})
	d.clearSyncStateRecoveryLocked()
	if err := d.replaceSyncBusLocked(bus, applyGate); err != nil {
		return nil, err
	}
	d.logger.Info("sync initialized", "server", candidate.serverURL, "device_id", candidate.state.DeviceID)
	return bus, nil
}

func (d *Daemon) replaceSyncBusLocked(next *forgedsync.Bus, applyGate *syncApplyGate) error {
	if next == nil {
		return fmt.Errorf("sync bus required")
	}
	if d.syncBus != nil && d.syncBus != next {
		return fmt.Errorf("sync bus replacement requires the previous bus to stop")
	}
	if d.syncDraining != nil {
		return fmt.Errorf("previous sync work is still stopping")
	}
	d.syncBus = next
	d.syncApplyGate = applyGate
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(next)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(next)
	}
	return nil
}

func (d *Daemon) detachSyncBusLocked() (*forgedsync.Bus, *syncApplyGate) {
	if d.syncBus == nil {
		return d.syncDraining, d.syncApplyGate
	}

	previous := d.syncBus
	applyGate := d.syncApplyGate
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
	return previous, applyGate
}

func (d *Daemon) drainSyncBus(bus *forgedsync.Bus) {
	bus.Wait()

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if d.syncDraining != bus {
		return
	}
	d.syncDraining = nil
	d.syncApplyGate = nil
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

	initRun := d.cancelSyncInitRunLocked()
	run := d.cancelLinkRunLocked()
	d.syncGeneration++
	d.syncPending = false
	d.syncRetryDelay = 0
	_, applyGate := d.detachSyncBusLocked()

	if d.routeService != nil {
		d.routeService.SetKeyStore(nil)
	}
	if d.ipcServer != nil {
		d.ipcServer.SetVaultState(nil, nil)
	}
	if d.agent != nil {
		d.agent.SetKeyStore(nil)
	}

	if initRun != nil && initRun.stateGate != nil {
		initRun.stateGate.revoke()
	}
	revokeSyncApplies(run, applyGate)
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

	if d.syncSuppressed || d.vault == nil {
		return
	}
	if d.syncBus != nil {
		d.syncBus.LocalMutation(reason)
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
	if runtime.GOOS == "windows" {
		// Windows PID files are advisory: a later unrelated process can reuse
		// the recorded PID. The per-user control pipe is the liveness authority.
		return 0, platform.IsSocketAlive(paths.CtlSocket())
	}

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
