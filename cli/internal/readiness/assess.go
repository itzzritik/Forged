package readiness

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type Engine struct {
	Paths config.Paths

	statPath        func(string) bool
	inspectService  func(config.Paths) (daemon.ServiceStatus, error)
	isRunning       func(config.Paths) (int, bool)
	socketReady     func(string) bool
	isSSHEnabled    func(config.Paths) bool
	detectOwner     func(config.Paths) (config.SSHAgentOwner, error)
	loadCredentials func(string) (bool, error)
	loadKeyCount    func(string) (int, error)
	ensureConfig    func(config.Paths) error
	enableSSH       func(config.Paths) error
	restartService  func() error
	sleep           func()
	serviceRetries  int
}

func New(paths config.Paths) *Engine {
	return &Engine{
		Paths:           paths,
		statPath:        fileExists,
		inspectService:  daemon.InspectService,
		isRunning:       daemon.IsRunning,
		socketReady:     defaultSocketReady,
		isSSHEnabled:    config.IsSSHAgentEnabled,
		detectOwner:     config.DetectSSHAgentOwner,
		loadCredentials: defaultCredentialsValid,
			loadKeyCount:    defaultLoadKeyCount,
			ensureConfig:    ensureDefaultConfigFile,
			enableSSH:       config.EnableSSHAgent,
			restartService:  daemon.RestartService,
			sleep: func() {
				time.Sleep(500 * time.Millisecond)
			},
			serviceRetries: 6,
		}
}

func (e *Engine) Assess() (Snapshot, error) {
	snapshot := Snapshot{
		VaultExists:        e.pathExists(e.Paths.VaultFile()),
		ConfigExists:       e.pathExists(e.Paths.ConfigFile()),
		ManagedConfigReady: e.pathExists(e.Paths.SSHManagedConfig()),
		SSHEnabled:         e.isSSH(e.Paths),
	}

	service, err := e.serviceStatus(e.Paths)
	if err != nil {
		service = daemon.DefaultServiceStatus()
		service.Repairable = false
		service.Detail = err.Error()
	}
	snapshot.Service = service

	if pid, running := e.running(e.Paths); running {
		snapshot.DaemonPID = pid
	}

	snapshot.IPCSocketReady = e.socketAlive(e.Paths.CtlSocket())
	snapshot.AgentSocketReady = e.socketAlive(e.Paths.AgentSocket())

	if owner, err := e.owner(e.Paths); err == nil {
		snapshot.IdentityAgentOwner = owner
	}

	if loggedIn, err := e.credentials(e.Paths.CredentialsFile()); err == nil {
		snapshot.LoggedIn = loggedIn
	}

	if snapshot.IPCSocketReady {
		if keyCount, err := e.keyCount(e.Paths.CtlSocket()); err == nil {
			snapshot.KeyCount = keyCount
		}
	}

	snapshot.State = classifyState(snapshot)
	return snapshot, nil
}

func classifyState(s Snapshot) State {
	if !s.VaultExists {
		return StateUninitialized
	}

	healthy := s.ConfigExists &&
		s.Service.Installed &&
		s.Service.ConfigValid &&
		s.Service.Running &&
		s.IPCSocketReady &&
		s.AgentSocketReady &&
		s.SSHEnabled &&
		s.ManagedConfigReady &&
		s.IdentityAgentOwner.IsForged()
	if healthy {
		if s.KeyCount == 0 {
			return StateReadyEmpty
		}
		return StateReady
	}

	if s.Service.Installed && (!s.Service.ConfigValid || !s.Service.Repairable) {
		return StateBlocked
	}

	if !s.Service.Installed && !s.ConfigExists && !s.SSHEnabled && !s.ManagedConfigReady {
		return StateSealed
	}

	return StateDegraded
}

func (e *Engine) pathExists(path string) bool {
	if e != nil && e.statPath != nil {
		return e.statPath(path)
	}
	return fileExists(path)
}

func (e *Engine) serviceStatus(paths config.Paths) (daemon.ServiceStatus, error) {
	if e != nil && e.inspectService != nil {
		return e.inspectService(paths)
	}
	return daemon.InspectService(paths)
}

func (e *Engine) running(paths config.Paths) (int, bool) {
	if e != nil && e.isRunning != nil {
		return e.isRunning(paths)
	}
	return daemon.IsRunning(paths)
}

func (e *Engine) socketAlive(path string) bool {
	if e != nil && e.socketReady != nil {
		return e.socketReady(path)
	}
	return defaultSocketReady(path)
}

func (e *Engine) isSSH(paths config.Paths) bool {
	if e != nil && e.isSSHEnabled != nil {
		return e.isSSHEnabled(paths)
	}
	return config.IsSSHAgentEnabled(paths)
}

func (e *Engine) owner(paths config.Paths) (config.SSHAgentOwner, error) {
	if e != nil && e.detectOwner != nil {
		return e.detectOwner(paths)
	}
	return config.DetectSSHAgentOwner(paths)
}

func (e *Engine) credentials(path string) (bool, error) {
	if e != nil && e.loadCredentials != nil {
		return e.loadCredentials(path)
	}
	return defaultCredentialsValid(path)
}

func (e *Engine) keyCount(socketPath string) (int, error) {
	if e != nil && e.loadKeyCount != nil {
		return e.loadKeyCount(socketPath)
	}
	return defaultLoadKeyCount(socketPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultSocketReady(path string) bool {
	if runtime.GOOS == "windows" {
		return false
	}

	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func defaultCredentialsValid(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var creds struct {
		Token     string `json:"token"`
		ServerURL string `json:"server_url"`
	}
	if err := json.Unmarshal(raw, &creds); err != nil {
		return false, err
	}

	return creds.Token != "" && creds.ServerURL != "", nil
}

func defaultLoadKeyCount(socketPath string) (int, error) {
	resp, err := ipc.NewClient(socketPath).Call(ipc.CmdStatus, nil)
	if err != nil {
		return 0, err
	}

	var result struct {
		KeyCount int `json:"key_count"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return 0, err
	}

	return result.KeyCount, nil
}

func ensureDefaultConfigFile(paths config.Paths) error {
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(paths.ConfigFile()); err == nil {
		return nil
	}

	content := fmt.Sprintf(`[agent]
socket = %q
log_level = "info"

[sync]
enabled = false
`, paths.AgentSocket())

	return os.WriteFile(paths.ConfigFile(), []byte(content), 0o600)
}
