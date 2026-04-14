package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
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
	paths        config.Paths
	vault        *vault.Vault
	keyStore     *vault.KeyStore
	activityLog  *activity.ActivityLog
	agent        *forgedagent.ForgedAgent
	agentServer  *forgedagent.Server
	ipcServer    *ipc.Server
	syncBus      *forgedsync.Bus
	authBroker   *sensitiveauth.Broker
	routingStore *sshrouting.Store
	routingState *sshrouting.State
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
		return fmt.Errorf("setting up logging: %w", err)
	}

	d.logger.Info("starting forged daemon")

	if err := d.cleanStaleState(); err != nil {
		return fmt.Errorf("cleaning stale state: %w", err)
	}

	if err := d.openVault(password); err != nil {
		return err
	}
	defer d.shutdown()

	if err := d.loadRoutingState(); err != nil {
		return fmt.Errorf("loading ssh routing state: %w", err)
	}

	d.authBroker = sensitiveauth.NewBroker(d.paths.VaultFile(), d.helperBinaryPath(), d.logger)

	if err := d.writePID(); err != nil {
		return err
	}

	d.activityLog = activity.NewActivityLog(1000)

	if err := d.startIPC(); err != nil {
		return err
	}

	d.initSync()

	if err := d.startAgent(); err != nil {
		return err
	}

	d.logger.Info("daemon ready",
		"keys", len(d.keyStore.List()),
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
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "forged-auth"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(exe), name)
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
			return fmt.Errorf("socket %s: %w", sock, err)
		}
	}

	pidPath := d.paths.PIDFile()
	if data, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(string(data)); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					return fmt.Errorf("daemon already running (PID %d)", pid)
				}
			}
		}
		os.Remove(pidPath)
	}

	return nil
}

func (d *Daemon) openVault(password []byte) error {
	vaultPath := d.paths.VaultFile()

	var v *vault.Vault
	var err error

	if _, statErr := os.Stat(vaultPath); os.IsNotExist(statErr) {
		d.logger.Info("creating new vault", "path", vaultPath)
		v, err = vault.Create(vaultPath, password)
	} else {
		d.logger.Info("opening vault", "path", vaultPath)
		v, err = vault.Open(vaultPath, password)
	}

	if err != nil {
		return fmt.Errorf("vault: %w", err)
	}

	if err := v.DecryptAllPrivateKeys(); err != nil {
		v.Close()
		return fmt.Errorf("hydrating private keys: %w", err)
	}

	d.vault = v
	d.keyStore = vault.NewKeyStore(v)

	for _, key := range v.Data.Keys {
		platform.Mlock(key.PrivateKey)
	}

	return nil
}

func (d *Daemon) loadRoutingState() error {
	d.routingStore = sshrouting.NewStore(d.paths.SSHRoutingStateFile())
	state, err := d.routingStore.Load()
	if err != nil {
		return err
	}
	d.routingState = state
	if len(d.routingState.Hosts) != 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, hint := range sshrouting.LegacyHints(d.keyStore.List()) {
		d.routingState.RecordSuccess(hint.Host, 22, hint.KeyID, now)
	}
	return d.routingStore.Save(d.routingState)
}

func (d *Daemon) writePID() error {
	pidPath := d.paths.PIDFile()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0700); err != nil {
		return fmt.Errorf("creating pid directory: %w", err)
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

func (d *Daemon) startIPC() error {
	ctlPath := d.paths.CtlSocket()
	if err := os.MkdirAll(filepath.Dir(ctlPath), 0700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	d.ipcServer = ipc.NewServer(ctlPath, d.vault, d.keyStore, d.activityLog, d.logger)
	d.ipcServer.SetSyncLinkHandler(d.handleSyncLink)
	d.ipcServer.SetSensitiveAuthBroker(d.authBroker)
	if err := d.ipcServer.Start(); err != nil {
		return fmt.Errorf("starting ipc server: %w", err)
	}

	d.logger.Info("ipc server started", "socket", ctlPath)
	return nil
}

func (d *Daemon) startAgent() error {
	agentPath := d.paths.AgentSocket()
	if err := os.MkdirAll(filepath.Dir(agentPath), 0700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	d.agent = forgedagent.New(d.keyStore)
	d.agent.SetSyncCoordinator(d.syncBus)
	d.agent.SetRouteCoordinator(d)
	d.agentServer = forgedagent.NewServer(agentPath, d.agent, d.logger)
	if err := d.agentServer.Start(); err != nil {
		return fmt.Errorf("starting agent server: %w", err)
	}

	d.logger.Info("ssh agent started", "socket", agentPath)
	return nil
}

type syncCredentials struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
}

func (d *Daemon) initSync() {
	data, err := os.ReadFile(d.paths.CredentialsFile())
	if err != nil {
		return
	}
	var creds syncCredentials
	if err := json.Unmarshal(data, &creds); err != nil || creds.ServerURL == "" || creds.Token == "" {
		return
	}

	state, err := d.configureSync(creds)
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
	stateStore := forgedsync.NewStateStore(d.paths.SyncStateFile())
	state, err := stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("loading sync state: %w", err)
	}
	if state == nil {
		defaultState := forgedsync.DefaultSyncState(uuid.NewString())
		state = &defaultState
	}
	if state.DeviceID == "" {
		state.DeviceID = uuid.NewString()
	}
	state.ServerURL = creds.ServerURL

	client := forgedsync.NewClient(creds.ServerURL, creds.Token, state.DeviceID)
	engine := forgedsync.NewEngine(d.vault, client, d.logger)
	bus := forgedsync.NewBus(engine, state, d.logger, forgedsync.BusConfig{
		DirtyFlagPath: d.paths.SyncDirtyFile(),
		StateStore:    stateStore,
	})

	d.syncBus = bus
	d.ipcServer.SetSyncBus(bus)
	if d.agent != nil {
		d.agent.SetSyncCoordinator(bus)
	}

	bus.CheckDirtyFlag()

	d.logger.Info("sync initialized", "server", creds.ServerURL, "device_id", state.DeviceID)
	return state, nil
}

func (d *Daemon) handleSyncLink(args ipc.SyncLinkArgs) error {
	if _, err := d.configureSync(syncCredentials{
		ServerURL: args.ServerURL,
		Token:     args.Token,
		UserID:    args.UserID,
	}); err != nil {
		return err
	}
	if d.syncBus == nil {
		return fmt.Errorf("sync bus unavailable")
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

	if d.syncBus != nil {
		d.syncBus.PersistDirtyFlag()
	}

	if d.agentServer != nil {
		d.agentServer.Stop()
	}

	if d.ipcServer != nil {
		d.ipcServer.Stop()
	}

	if d.authBroker != nil {
		d.authBroker.Close()
	}

	if d.vault != nil {
		for _, key := range d.vault.Data.Keys {
			for i := range key.PrivateKey {
				key.PrivateKey[i] = 0
			}
			platform.Munlock(key.PrivateKey)
		}
		d.vault.Close()
	}

	os.Remove(d.paths.AgentSocket())
	os.Remove(d.paths.CtlSocket())
	os.Remove(d.paths.PIDFile())

	d.logger.Info("daemon stopped")
}

func (d *Daemon) OrderedKeys(keys []vault.Key) []vault.Key {
	ordered := append([]vault.Key(nil), keys...)
	sort.SliceStable(ordered, func(i, j int) bool {
		score := func(k vault.Key) int {
			if d.routingState == nil {
				return 0
			}
			best := 0
			for _, entry := range d.routingState.Hosts {
				if entry.KeyID == k.ID && entry.SuccessCount > best {
					best = entry.SuccessCount
				}
			}
			return best
		}

		si, sj := score(ordered[i]), score(ordered[j])
		if si != sj {
			return si > sj
		}

		iUsed, jUsed := ordered[i].LastUsedAt, ordered[j].LastUsedAt
		if iUsed != nil && jUsed != nil {
			return iUsed.After(*jUsed)
		}
		if iUsed != nil {
			return true
		}
		if jUsed != nil {
			return false
		}
		return ordered[i].Name < ordered[j].Name
	})
	return ordered
}

func IsRunning(paths config.Paths) (int, bool) {
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(string(data))
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
	return pid, true
}
