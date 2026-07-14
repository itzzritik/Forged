package daemon

import (
	"context"
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
	sessionMu      sync.Mutex
	paths          config.Paths
	vault          *vault.Vault
	keyStore       *vault.KeyStore
	activityLog    *activity.ActivityLog
	agent          *forgedagent.ForgedAgent
	agentServer    *forgedagent.Server
	ipcServer      *ipc.Server
	syncBus        *forgedsync.Bus
	authBroker     *sensitiveauth.Broker
	sshRouting     *sshrouting.Manager
	routeService   *sshrouting.Service
	syncGeneration uint64
	syncPending    bool
	syncSuppressed bool
	syncRetryDelay time.Duration
	logger         *slog.Logger
	stop           chan struct{}
}

func New(paths config.Paths) *Daemon {
	return &Daemon{
		paths: paths,
		stop:  make(chan struct{}),
	}
}

func (d *Daemon) Run(password []byte) error {
	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("Setting up logging: %w", err)
	}

	d.logger.Info("starting forged daemon")

	if err := d.cleanStaleState(); err != nil {
		return fmt.Errorf("Cleaning stale state: %w", err)
	}

	defer d.shutdown()

	d.routeService = sshrouting.NewService(d.paths, nil)
	d.routeService.SetOnMutation(d.handleRouteMutation)
	d.sshRouting = sshrouting.NewManager(d.paths, d.selfBinaryPath())
	d.authBroker = sensitiveauth.NewBroker(d.paths, d.helperBinaryPath(), d.logger, d)

	if len(password) > 0 {
		if err := d.hydrateWithPassword(password); err != nil {
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
	close(d.stop)
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
	for _, sock := range []string{d.paths.AgentSocket(), d.paths.CtlSocket()} {
		if err := platform.CleanStaleSocket(sock); err != nil {
			return fmt.Errorf("Socket %s: %w", sock, err)
		}
	}

	pidPath := d.paths.PIDFile()
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
	if err := os.MkdirAll(filepath.Dir(ctlPath), 0700); err != nil {
		return fmt.Errorf("Creating socket directory: %w", err)
	}

	d.ipcServer = ipc.NewServer(ctlPath, d.vault, d.keyStore, d.activityLog, d.logger)
	d.ipcServer.SetSyncLinkHandler(d.handleSyncLink)
	d.ipcServer.SetSyncUnlinkHandler(d.handleSyncUnlink)
	d.ipcServer.SetAccountReplaceHandler(d.handleAccountReplace)
	d.ipcServer.SetAccountClearHandler(d.handleAccountClear)
	d.ipcServer.SetSensitiveAuthBroker(d.authBroker)
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
	d.ipcServer.SetSSHRouteHandler(d.routeService)
	if err := d.ipcServer.Start(); err != nil {
		return fmt.Errorf("Starting IPC server: %w", err)
	}

	d.logger.Info("ipc server started", "socket", ctlPath)
	return nil
}

func (d *Daemon) startAgentLocked() error {
	agentPath := d.paths.AgentSocket()
	if err := os.MkdirAll(filepath.Dir(agentPath), 0700); err != nil {
		return fmt.Errorf("Creating socket directory: %w", err)
	}

	d.agent = forgedagent.New(d.keyStore)
	d.agent.SetSyncCoordinator(d.syncBus)
	d.agent.SetRouteSessions(d.routeService)
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

const syncLinkTimeout = 25 * time.Second

func (d *Daemon) initSyncLocked() {
	if d.vault == nil || d.syncSuppressed || d.syncPending {
		return
	}
	if d.syncBus != nil {
		return
	}

	creds, err := accountauth.Load(d.paths)
	if err != nil || creds.ServerURL == "" || accountauth.CurrentToken(creds) == "" {
		return
	}
	if err := d.recoverSyncStateLocked(creds); err != nil {
		d.logger.Warn("recovering sync state transaction failed", "error", err)
		d.scheduleSyncRetryLocked(d.syncGeneration)
		return
	}

	candidate, err := d.prepareSyncCandidate(syncCredentials{
		ServerURL: creds.ServerURL,
		UserID:    creds.UserID,
		Token:     accountauth.CurrentToken(creds),
	})
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		return
	}
	d.syncGeneration++
	candidate.generation = d.syncGeneration

	if creds.UserID != "" && (candidate.state.LinkedUserID != creds.UserID || (candidate.state.LastKnownServerVersion == 0 && len(candidate.state.LastSyncedBaseBlob) == 0)) {
		if err := d.markSyncDirtyLocked(); err != nil {
			d.logger.Warn("initializing sync failed", "error", err)
			return
		}
		d.syncPending = true
		go d.finishInitialSync(candidate, creds.UserID)
		return
	}

	var bus *forgedsync.Bus
	err = accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
		if err := validateSyncIdentity(stored, candidate.serverURL, candidate.userID); err != nil {
			return err
		}
		var err error
		bus, err = d.activateSyncCandidateLocked(candidate)
		return err
	})
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		return
	}
	go bus.LifecycleRefresh("daemon_start")
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

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if err := d.beginAccountChangeLocked(); err != nil {
		return err
	}
	if _, err := d.requireSyncIdentity(args.ServerURL, args.UserID); err != nil {
		d.syncSuppressed = false
		d.initSyncLocked()
		return err
	}
	d.syncSuppressed = false
	d.initSyncLocked()
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

	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if err := d.beginAccountChangeLocked(); err != nil {
		return err
	}
	if current, err := accountauth.Load(d.paths); err == nil {
		if err := d.recoverSyncStateLocked(current); err != nil {
			d.syncSuppressed = false
			d.initSyncLocked()
			return err
		}
	}
	staged, err := d.stageSyncStateLocked()
	if err != nil {
		d.syncSuppressed = false
		d.initSyncLocked()
		return err
	}
	if err := accountauth.Save(d.paths, creds); err != nil {
		if restoreErr := d.restoreSyncStateLocked(staged); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
		d.syncSuppressed = false
		d.initSyncLocked()
		return err
	}
	if err := d.removeSyncStateBackupLocked(); err != nil {
		d.logger.Warn("removing staged sync state after account replacement failed", "error", err)
	}
	d.syncSuppressed = false
	d.initSyncLocked()
	d.logger.Info("account replacement committed", "user_id", args.UserID)
	return nil
}

func (d *Daemon) handleAccountClear() error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	if err := d.beginAccountChangeLocked(); err != nil {
		return err
	}
	creds, _ := accountauth.Load(d.paths)
	if err := accountauth.Delete(d.paths); err != nil {
		d.syncSuppressed = false
		d.initSyncLocked()
		return err
	}
	if err := d.removeSyncStateLocked(); err != nil {
		d.logger.Warn("removing sync state after logout failed", "error", err)
	}
	_ = os.Remove(d.paths.SyncDirtyFile())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := accountauth.RevokeRemoteSession(ctx, creds); err != nil {
		d.logger.Warn("revoking remote session after logout failed", "error", err)
	}
	d.logger.Info("account cleared")
	return nil
}

func (d *Daemon) beginAccountChangeLocked() error {
	if d.vault != nil && d.keyStore != nil {
		if err := d.markSyncDirtyLocked(); err != nil {
			return err
		}
		if d.syncBus != nil {
			d.syncBus.LocalMutation("account_link_transition")
		}
	}
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0
	d.replaceSyncBusLocked(nil)
	return nil
}

func (d *Daemon) removeSyncStateLocked() error {
	var removeErr error
	if err := os.Remove(d.paths.SyncStateFile()); err != nil && !os.IsNotExist(err) {
		removeErr = errors.Join(removeErr, fmt.Errorf("Removing stale sync state: %w", err))
	}
	return errors.Join(removeErr, d.removeSyncStateBackupLocked())
}

func (d *Daemon) syncStateBackupPath() string {
	return d.paths.SyncStateFile() + ".account-change"
}

func (d *Daemon) removeSyncStateBackupLocked() error {
	if err := os.Remove(d.syncStateBackupPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Removing staged sync state: %w", err)
	}
	return nil
}

func (d *Daemon) stageSyncStateLocked() (bool, error) {
	_, stateErr := os.Stat(d.paths.SyncStateFile())
	stateExists := stateErr == nil
	if stateErr != nil && !os.IsNotExist(stateErr) {
		return false, fmt.Errorf("Reading sync state: %w", stateErr)
	}
	_, backupErr := os.Stat(d.syncStateBackupPath())
	backupExists := backupErr == nil
	if backupErr != nil && !os.IsNotExist(backupErr) {
		return false, fmt.Errorf("Reading staged sync state: %w", backupErr)
	}
	if backupExists {
		if stateExists {
			return false, fmt.Errorf("Active and staged sync state both exist")
		}
		return true, nil
	}
	if !stateExists {
		return false, nil
	}
	if err := os.Rename(d.paths.SyncStateFile(), d.syncStateBackupPath()); err != nil {
		return false, fmt.Errorf("Staging sync state: %w", err)
	}
	return true, nil
}

func (d *Daemon) restoreSyncStateLocked(staged bool) error {
	if !staged {
		return nil
	}
	_ = os.Remove(d.paths.SyncStateFile())
	if err := os.Rename(d.syncStateBackupPath(), d.paths.SyncStateFile()); err != nil {
		return fmt.Errorf("Restoring sync state: %w", err)
	}
	return nil
}

func (d *Daemon) recoverSyncStateLocked(creds accountauth.Credentials) error {
	backup := d.syncStateBackupPath()
	state, err := forgedsync.NewStateStore(backup).Load()
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	matches := (state.LinkedUserID == "" || state.LinkedUserID == creds.UserID) &&
		(state.ServerURL == "" || sameSyncServer(state.ServerURL, creds.ServerURL))
	if !matches {
		if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Removing staged sync state for another account: %w", err)
		}
		return nil
	}
	_ = os.Remove(d.paths.SyncStateFile())
	return os.Rename(backup, d.paths.SyncStateFile())
}

func (d *Daemon) handleSyncUnlink() error {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()
	d.syncGeneration++
	d.syncPending = false
	d.syncSuppressed = true
	d.syncRetryDelay = 0

	if d.syncBus != nil {
		if err := d.syncBus.AuthUnlinked(context.Background()); err != nil {
			return err
		}
	}
	d.replaceSyncBusLocked(nil)

	if err := d.removeSyncStateLocked(); err != nil {
		return err
	}
	if err := os.Remove(d.paths.SyncDirtyFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Removing sync dirty flag: %w", err)
	}

	d.logger.Info("sync unlinked")
	return nil
}

func (d *Daemon) finishInitialSync(candidate *syncCandidate, userID string) {
	d.sessionMu.Lock()
	defer d.sessionMu.Unlock()

	if candidate.generation != d.syncGeneration || d.syncSuppressed {
		return
	}
	d.syncPending = false
	if d.vault == nil || d.vault != candidate.vault || d.syncBus != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), syncLinkTimeout)
	defer cancel()
	refreshed, err := accountauth.EnsureFresh(ctx, d.paths)
	if err == nil {
		err = validateSyncIdentity(refreshed, candidate.serverURL, userID)
	}
	var bus *forgedsync.Bus
	if err == nil {
		err = accountauth.WithCredentials(d.paths, func(stored accountauth.Credentials) error {
			if err := validateSyncIdentity(stored, candidate.serverURL, userID); err != nil {
				return err
			}
			client := forgedsync.NewClient(candidate.serverURL, accountauth.CurrentToken(stored), candidate.state.DeviceID)
			candidate.engine = forgedsync.NewEngine(candidate.vault, client, d.logger)
			if err := candidate.engine.ReconcileOnLink(ctx, candidate.state, userID, candidate.serverURL); err != nil {
				return err
			}
			var err error
			bus, err = d.activateSyncCandidateLocked(candidate)
			return err
		})
	}
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
	d.replaceSyncBusLocked(bus)
	d.logger.Info("sync initialized", "server", candidate.serverURL, "device_id", candidate.state.DeviceID)
	return bus, nil
}

func (d *Daemon) replaceSyncBusLocked(next *forgedsync.Bus) {
	previous := d.syncBus
	if previous != nil && previous != next {
		d.syncBus = nil
		if d.ipcServer != nil {
			d.ipcServer.SetSyncBus(nil)
		}
		if d.agent != nil {
			d.agent.SetSyncCoordinator(nil)
		}
		previous.Stop()
	}
	d.syncBus = next
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(next)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(next)
	}
}

func (d *Daemon) syncTokenSource(serverURL, userID string) func() (string, error) {
	return func() (string, error) {
		creds, err := accountauth.EnsureFresh(context.Background(), d.paths)
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

	d.syncGeneration++
	d.syncPending = false
	d.syncRetryDelay = 0
	d.replaceSyncBusLocked(nil)

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
	d.logger.Info("shutting down")

	if d.agentServer != nil {
		d.agentServer.Stop()
	}

	if d.ipcServer != nil {
		d.ipcServer.Stop()
	}

	if d.authBroker != nil {
		d.authBroker.Close()
	}
	d.clearActiveSession("shutdown")

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
