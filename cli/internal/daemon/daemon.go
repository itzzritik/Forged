package daemon

import (
	"context"
	"fmt"
	"io"
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
	sessionMu    sync.Mutex
	paths        config.Paths
	vault        *vault.Vault
	keyStore     *vault.KeyStore
	activityLog  *activity.ActivityLog
	agent        *forgedagent.ForgedAgent
	agentServer  *forgedagent.Server
	ipcServer    *ipc.Server
	syncBus      *forgedsync.Bus
	authBroker   *sensitiveauth.Broker
	sshRouting   *sshrouting.Manager
	routeService *sshrouting.Service
	logger       *slog.Logger
	stop         chan struct{}
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

	if err := d.writePID(); err != nil {
		return err
	}

	d.activityLog = activity.NewActivityLog(1000)

	if err := d.startIPC(); err != nil {
		return err
	}

	if err := d.refreshSSHRouting(); err != nil {
		return err
	}

	if err := d.startAgent(); err != nil {
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

	w := io.MultiWriter(os.Stderr, lj)
	d.logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
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
	if data, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
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
	d.ipcServer.SetSensitiveAuthBroker(d.authBroker)
	if d.syncBus != nil {
		d.ipcServer.SetSyncBus(d.syncBus)
	}
	d.ipcServer.SetOnKeyChange(func() {
		if err := d.refreshSSHRouting(); err != nil {
			d.logger.Warn("refreshing ssh routing after key change failed", "error", err)
		}
	})
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

func (d *Daemon) startAgent() error {
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
}

func (d *Daemon) initSync() {
	if d.vault == nil {
		return
	}
	if d.syncBus != nil {
		return
	}

	creds, err := accountauth.Load(d.paths)
	if err != nil || creds.ServerURL == "" || accountauth.CurrentToken(creds) == "" {
		return
	}

	state, err := d.configureSync(syncCredentials{
		ServerURL: creds.ServerURL,
		UserID:    creds.UserID,
	})
	if err != nil {
		d.logger.Warn("initializing sync failed", "error", err)
		return
	}

	if creds.UserID != "" && (state.LinkedUserID != creds.UserID || (state.LastKnownServerVersion == 0 && len(state.LastSyncedBaseBlob) == 0)) {
		go func() {
			if err := d.syncBus.AuthLinked(context.Background(), creds.UserID, creds.ServerURL); err != nil {
				d.logger.Warn("link reconcile failed", "error", err)
			}
		}()
		return
	}

	go d.syncBus.LifecycleRefresh("daemon_start")
}

func (d *Daemon) configureSync(creds syncCredentials) (*forgedsync.SyncState, error) {
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
	state.ServerURL = creds.ServerURL

	client := forgedsync.NewClientWithTokenSource(creds.ServerURL, state.DeviceID, func() (string, error) {
		creds, err := accountauth.EnsureFresh(context.Background(), d.paths)
		if err != nil {
			return "", fmt.Errorf("Refreshing account credentials: %w", err)
		}
		return accountauth.CurrentToken(creds), nil
	})
	engine := forgedsync.NewEngine(d.vault, client, d.logger)
	bus := forgedsync.NewBus(engine, state, d.logger, forgedsync.BusConfig{
		DirtyFlagPath: d.paths.SyncDirtyFile(),
		StateStore:    stateStore,
	})

	if d.syncBus != nil {
		d.syncBus.Stop()
	}
	d.syncBus = bus
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(bus)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(bus)
	}

	bus.CheckDirtyFlag()

	d.logger.Info("sync initialized", "server", creds.ServerURL, "device_id", state.DeviceID)
	return state, nil
}

func (d *Daemon) handleSyncLink(args ipc.SyncLinkArgs) error {
	if !d.HasActiveSession() {
		return fmt.Errorf("Vault is locked; open Forged to unlock")
	}
	if _, err := d.configureSync(syncCredentials{
		ServerURL: args.ServerURL,
		UserID:    args.UserID,
	}); err != nil {
		return err
	}
	if d.syncBus == nil {
		return fmt.Errorf("Sync bus unavailable")
	}
	if args.UserID == "" {
		return nil
	}
	if err := d.syncBus.AuthLinked(context.Background(), args.UserID, args.ServerURL); err != nil {
		return err
	}
	d.logger.Info("sync link refreshed", "user_id", args.UserID)
	return nil
}

func (d *Daemon) handleSyncUnlink() error {
	if d.syncBus != nil {
		if err := d.syncBus.AuthUnlinked(context.Background()); err != nil {
			return err
		}
		d.syncBus.Stop()
	}

	d.syncBus = nil
	if d.ipcServer != nil {
		d.ipcServer.SetSyncBus(nil)
	}
	if d.agent != nil {
		d.agent.SetSyncCoordinator(nil)
	}

	if err := os.Remove(d.paths.SyncStateFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Removing sync state: %w", err)
	}
	if err := os.Remove(d.paths.SyncDirtyFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Removing sync dirty flag: %w", err)
	}

	d.logger.Info("sync unlinked")
	return nil
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
	return d.hydrateWithSymmetricKey(symmetricKey, "local_enrollment")
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

	if d.routeService != nil {
		d.routeService.SetKeyStore(keyStore)
	}
	if d.ipcServer != nil {
		d.ipcServer.SetVaultState(v, keyStore)
	}
	if d.agent != nil {
		d.agent.SetKeyStore(keyStore)
	}

	d.initSync()
	if err := d.refreshSSHRouting(); err != nil && d.logger != nil {
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

	if d.syncBus != nil {
		d.syncBus.PersistDirtyFlag()
		d.syncBus.Stop()
		d.syncBus = nil
		if d.ipcServer != nil {
			d.ipcServer.SetSyncBus(nil)
		}
		if d.agent != nil {
			d.agent.SetSyncCoordinator(nil)
		}
	}

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
	bus := d.syncBus
	d.sessionMu.Unlock()

	if bus != nil {
		bus.LocalMutation(reason)
	}
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

	os.Remove(d.paths.AgentSocket())
	os.Remove(d.paths.CtlSocket())
	os.Remove(d.paths.PIDFile())

	d.logger.Info("daemon stopped")
}

func zeroSecret(b []byte) {
	for i := range b {
		b[i] = 0
	}
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
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
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
