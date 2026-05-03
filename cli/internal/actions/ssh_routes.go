package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type SSHRoutingDebug struct {
	Routes          []SSHRouteDebug          `json:"routes"`
	RuntimeAttempts []SSHRouteRuntimeAttempt `json:"runtime_attempts"`
	PublicHints     []SSHRoutePublicHint     `json:"public_hints"`
}

type SSHRouteDebug struct {
	Target        string                 `json:"target"`
	Kind          string                 `json:"kind,omitempty"`
	Host          string                 `json:"host,omitempty"`
	User          string                 `json:"user,omitempty"`
	Port          int                    `json:"port,omitempty"`
	Owner         string                 `json:"owner,omitempty"`
	Repo          string                 `json:"repo,omitempty"`
	Fingerprint   string                 `json:"fingerprint,omitempty"`
	KeyName       string                 `json:"key_name,omitempty"`
	KeyRef        string                 `json:"key_ref,omitempty"`
	ProvenBy      string                 `json:"proven_by,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	SuccessCount  int                    `json:"success_count,omitempty"`
	LastSuccessAt *time.Time             `json:"last_success_at,omitempty"`
	Updated       time.Time              `json:"updated"`
	Attempts      []SSHRouteDebugAttempt `json:"attempts,omitempty"`
}

type SSHRouteDebugAttempt struct {
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	KeyRef      string    `json:"key_ref,omitempty"`
	AttemptedAt time.Time `json:"attempted_at"`
}

type SSHRouteRuntimeAttempt struct {
	Token         string                  `json:"token"`
	Path          string                  `json:"path"`
	Updated       time.Time               `json:"updated"`
	AgeSeconds    int64                   `json:"age_seconds"`
	IdentityFiles []SSHRouteIdentityFile  `json:"identity_files,omitempty"`
	Clients       []SSHRouteRuntimeClient `json:"clients,omitempty"`
}

type SSHRouteRuntimeClient struct {
	ClientPID  int      `json:"client_pid,omitempty"`
	Target     string   `json:"target,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	HadExact   bool     `json:"had_exact,omitempty"`
}

type SSHRouteIdentityFile struct {
	Path        string `json:"path"`
	Ref         string `json:"ref,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	KeyName     string `json:"key_name,omitempty"`
}

type SSHRoutePublicHint struct {
	Ref         string    `json:"ref"`
	Path        string    `json:"path"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	KeyName     string    `json:"key_name,omitempty"`
	Updated     time.Time `json:"updated"`
	Stale       bool      `json:"stale,omitempty"`
}

func LoadSSHRoutingDebug(paths config.Paths) (SSHRoutingDebug, error) {
	snapshot, err := loadSSHRoutingDebug(paths)
	if err == nil {
		return snapshot, nil
	}
	if !isUnknownIPCCommand(err, ipc.CmdSSHRoutesList) && !isDaemonUnavailable(err) {
		return SSHRoutingDebug{}, err
	}

	if refreshErr := refreshDaemonForSSHRoutingDebug(paths); refreshErr != nil {
		return SSHRoutingDebug{}, fmt.Errorf("refreshing daemon for SSH Routing Lab: %w", refreshErr)
	}
	unlock, unlockErr := UnlockSensitiveLaunch(paths, nil)
	if unlockErr != nil {
		return SSHRoutingDebug{}, fmt.Errorf("unlocking refreshed daemon for SSH Routing Lab: %w", unlockErr)
	}
	if unlock.PasswordRequired {
		return SSHRoutingDebug{}, fmt.Errorf("daemon was refreshed; unlock Forged again to load vault-backed route memory")
	}

	snapshot, err = loadSSHRoutingDebug(paths)
	if err != nil {
		return SSHRoutingDebug{}, fmt.Errorf("loading SSH routing debug after daemon refresh: %w", err)
	}
	return snapshot, nil
}

func loadSSHRoutingDebug(paths config.Paths) (SSHRoutingDebug, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRoutesList, nil)
	if err != nil {
		return SSHRoutingDebug{}, err
	}
	var snapshot SSHRoutingDebug
	if err := json.Unmarshal(resp.Data, &snapshot); err != nil {
		return SSHRoutingDebug{}, fmt.Errorf("parsing SSH routing debug payload: %w", err)
	}
	return snapshot, nil
}

func refreshDaemonForSSHRoutingDebug(paths config.Paths) error {
	if pid, running := daemon.IsRunning(paths); running {
		if err := stopForgedDaemon(pid); err != nil {
			return err
		}
		if err := waitForSSHRoutingDebugReady(paths, 2*time.Second); err == nil {
			return nil
		}
	}

	if err := startDirectDaemon(paths); err != nil {
		return err
	}
	return waitForDaemonStatus(paths, 6*time.Second)
}

func stopForgedDaemon(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding daemon process %d: %w", pid, err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("stopping daemon process %d: %w", pid, err)
	}
	return nil
}

func startDirectDaemon(paths config.Paths) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}

	if err := os.MkdirAll(filepath.Dir(paths.LogFile()), 0700); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}
	logFile, err := os.OpenFile(paths.LogFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening daemon log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "daemon")
	cmd.SysProcAttr = detachedDaemonProcAttr()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}
	return cmd.Process.Release()
}

func waitForSSHRoutingDebugReady(paths config.Paths, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := loadSSHRoutingDebug(paths); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("waiting for SSH routing debug")
}

func waitForDaemonStatus(paths config.Paths, timeout time.Duration) error {
	client := ipc.NewClient(paths.CtlSocket())
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := client.Call(ipc.CmdStatus, nil); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("waiting for daemon status: %w", lastErr)
	}
	return fmt.Errorf("waiting for daemon status")
}

func isUnknownIPCCommand(err error, command string) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "unknown command") && strings.Contains(message, strings.ToLower(command))
}

func isDaemonUnavailable(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "daemon is not running")
}

func ClearSSHRoute(paths config.Paths, target string) error {
	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRouteClear, ipc.SSHRouteClearArgs{Target: target})
	return err
}

func ClearAllSSHRoutes(paths config.Paths) error {
	_, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdSSHRoutesClearAll, nil)
	return err
}
