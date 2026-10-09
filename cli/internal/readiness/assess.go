package readiness

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
)

type DaemonRuntimeStatus struct {
	KeyCount int
	BuildID  string
}

type Engine struct {
	Paths config.Paths

	statPath         func(string) bool
	inspectService   func(config.Paths) (daemon.ServiceStatus, error)
	isRunning        func(config.Paths) (int, bool)
	socketReady      func(string) bool
	isSSHEnabled     func(config.Paths) bool
	isManagedSSH     func(config.Paths) bool
	detectOwner      func(config.Paths) (config.SSHAgentOwner, error)
	inspectGitSSH    func() config.GitSSHStatus
	fixGitSSH        func() error
	loadCredentials  func(config.Paths) (bool, error)
	loadDaemonStatus func(string) (DaemonRuntimeStatus, error)
	ensureConfig     func(config.Paths) error
	enableSSH        func(config.Paths) error
	ensureService    func(config.Paths, daemon.RuntimeSpec) error
	serviceRuntime   func() (daemon.RuntimeSpec, error)
	sleep            func()
	serviceRetries   int
}

func New(paths config.Paths) *Engine {
	return &Engine{
		Paths:            paths,
		statPath:         fileExists,
		inspectService:   daemon.InspectService,
		isRunning:        daemon.IsRunning,
		socketReady:      defaultSocketReady,
		isSSHEnabled:     config.IsSSHAgentEnabled,
		isManagedSSH:     config.IsManagedSSHIntegrationEnabled,
		detectOwner:      config.DetectSSHAgentOwner,
		inspectGitSSH:    config.InspectGitSSH,
		fixGitSSH:        config.EnsureGitUsesNativeSSH,
		loadCredentials:  defaultCredentialsValid,
		loadDaemonStatus: defaultDaemonRuntimeStatus,
		ensureConfig:     ensureDefaultConfigFile,
		enableSSH:        config.EnableSSHAgent,
		ensureService:    daemon.EnsureService,
		serviceRuntime:   daemon.DefaultRuntimeSpec,
		sleep: func() {
			time.Sleep(500 * time.Millisecond)
		},
		serviceRetries: 17,
	}
}

func (e *Engine) Assess() (Snapshot, error) {
	snapshot := Snapshot{
		VaultExists:        e.pathExists(e.Paths.VaultFile()),
		ConfigExists:       e.pathExists(e.Paths.ConfigFile()),
		ManagedConfigReady: e.pathExists(e.Paths.SSHManagedConfig()),
		AgentDisabled:      config.IsAgentDisabled(e.Paths),
		CurrentBuildID:     buildinfo.CurrentID(),
		Linger:             daemon.InspectLinger(),
	}
	if snapshot.ConfigExists {
		if _, err := config.Load(e.Paths.ConfigFile()); err != nil {
			snapshot.ConfigError = err.Error()
		} else {
			snapshot.ConfigValid = true
		}
	}
	if loggedIn, err := e.credentials(e.Paths); err == nil {
		snapshot.LoggedIn = loggedIn
	} else {
		snapshot.LoginCheckError = accountauth.CredentialLoadDiagnostic(err)
	}

	if err := e.Paths.ValidateRuntimePaths(); err != nil {
		snapshot.RuntimePathError = fmt.Sprintf("resolving runtime socket paths: %v", err)
		snapshot.State = classifyState(snapshot)
		return snapshot, nil
	}

	snapshot.SSHEnabled = e.isSSH(e.Paths)
	snapshot.ManagedSSHIntegration = e.isManagedSSHIntegrationEnabled(e.Paths)

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
	snapshot.GitSSH = e.gitSSH()
	snapshot.GitSigningStale = config.GitSigningProgramStale(daemon.InstalledBinary("forged-sign"))

	if snapshot.IPCSocketReady {
		if status, err := e.daemonStatus(e.Paths.CtlSocket()); err == nil {
			snapshot.KeyCount = status.KeyCount
			snapshot.DaemonBuildID = status.BuildID
		} else if errors.Is(err, ipc.ErrDaemonIdentity) && !errors.Is(err, os.ErrNotExist) {
			snapshot.RuntimePathError = fmt.Sprintf("untrusted daemon endpoint: %v", err)
		}
	}

	snapshot.State = classifyState(snapshot)
	return snapshot, nil
}

func classifyState(s Snapshot) State {
	if strings.TrimSpace(s.RuntimePathError) != "" {
		return StateBlocked
	}
	if s.ConfigExists && !s.ConfigValid {
		return StateBlocked
	}
	if !s.VaultExists {
		if strings.TrimSpace(s.LoginCheckError) != "" {
			return StateBlocked
		}
		return StateUninitialized
	}

	healthy := s.ConfigExists &&
		s.ConfigValid &&
		s.Service.Installed &&
		s.Service.ConfigValid &&
		s.Service.Running &&
		serviceOwnsDaemon(s) &&
		serviceBuildFresh(s) &&
		serviceRunsInstalledCopy(s) &&
		!s.GitSigningStale &&
		s.IPCSocketReady &&
		s.AgentSocketReady &&
		(s.AgentDisabled || s.SSHHealthy())
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

func serviceOwnsDaemon(s Snapshot) bool {
	if !s.Service.PIDKnown {
		return true
	}
	return s.DaemonPID > 0 && s.Service.PID == s.DaemonPID
}

func serviceBuildFresh(s Snapshot) bool {
	current := strings.TrimSpace(s.CurrentBuildID)
	if current == "" {
		return true
	}
	return strings.TrimSpace(s.DaemonBuildID) == current
}

// Older installs ran the service straight from the npm install.
func serviceRunsInstalledCopy(s Snapshot) bool {
	return daemon.InInstallDir(s.Service.BinaryPath)
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

func (e *Engine) isManagedSSHIntegrationEnabled(paths config.Paths) bool {
	if e != nil && e.isManagedSSH != nil {
		return e.isManagedSSH(paths)
	}
	return config.IsManagedSSHIntegrationEnabled(paths)
}

func (e *Engine) owner(paths config.Paths) (config.SSHAgentOwner, error) {
	if e != nil && e.detectOwner != nil {
		return e.detectOwner(paths)
	}
	return config.DetectSSHAgentOwner(paths)
}

func (e *Engine) gitSSH() config.GitSSHStatus {
	if e != nil && e.inspectGitSSH != nil {
		return e.inspectGitSSH()
	}
	return config.InspectGitSSH()
}

func (e *Engine) fixGitSSHConfig() error {
	if e != nil && e.fixGitSSH != nil {
		return e.fixGitSSH()
	}
	return config.EnsureGitUsesNativeSSH()
}

func (e *Engine) credentials(paths config.Paths) (bool, error) {
	if e != nil && e.loadCredentials != nil {
		return e.loadCredentials(paths)
	}
	return defaultCredentialsValid(paths)
}

func (e *Engine) daemonStatus(socketPath string) (DaemonRuntimeStatus, error) {
	if e != nil && e.loadDaemonStatus != nil {
		return e.loadDaemonStatus(socketPath)
	}
	return defaultDaemonRuntimeStatus(socketPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultSocketReady(path string) bool {
	return platform.IsSocketAlive(path)
}

func defaultCredentialsValid(paths config.Paths) (bool, error) {
	creds, err := accountauth.Load(paths)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, accountauth.ErrLoginRequired) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(creds.ServerURL) != "" &&
		(accountauth.CurrentToken(creds) != "" || accountauth.CanRefresh(creds, time.Now())), nil
}

func defaultDaemonRuntimeStatus(socketPath string) (DaemonRuntimeStatus, error) {
	resp, err := ipc.NewClient(socketPath).Call(ipc.CmdStatus, nil)
	if err != nil {
		return DaemonRuntimeStatus{}, err
	}

	var result struct {
		KeyCount int    `json:"key_count"`
		BuildID  string `json:"build_id"`
		Build    struct {
			ID string `json:"id"`
		} `json:"build"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return DaemonRuntimeStatus{}, err
	}

	buildID := strings.TrimSpace(result.BuildID)
	if buildID == "" {
		buildID = strings.TrimSpace(result.Build.ID)
	}
	return DaemonRuntimeStatus{KeyCount: result.KeyCount, BuildID: buildID}, nil
}

func ensureDefaultConfigFile(paths config.Paths) error {
	return config.EnsureDefault(paths)
}
