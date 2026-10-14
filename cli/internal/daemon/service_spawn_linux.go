//go:build linux

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

// Without a systemd user manager (WSL, containers, OpenRC) the daemon runs as a background process
// started by forged. Its PID and start identity in daemon.spawn make it ours, like a service PID;
// nothing restarts it, so the next forged launch starts it again.
var systemdUserManager = sync.OnceValue(func() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "show-environment")
	cmd.Env = systemdUserEnv()
	// A manager that hangs is still there; a background daemon beside it would race its own.
	return cmd.Run() == nil || ctx.Err() != nil
})

func spawnRecordPath() string {
	return filepath.Join(config.DefaultPaths().RuntimeDir, "daemon.spawn")
}

func spawnedDaemon() (platform.ProcessInstance, bool) {
	var instance platform.ProcessInstance
	data, err := os.ReadFile(spawnRecordPath())
	if err != nil || json.Unmarshal(data, &instance) != nil {
		return instance, false
	}
	same, err := platform.SameProcessInstance(instance)
	return instance, err == nil && same && !zombie(instance.PID)
}

// zombie reports an exited child nobody reaped yet; container PID 1 often never reaps.
func zombie(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	_, rest, ok := strings.Cut(string(data), ") ")
	return ok && strings.HasPrefix(rest, "Z")
}

func startSpawnedDaemon() error {
	if _, running := spawnedDaemon(); running {
		return nil
	}
	paths := config.DefaultPaths()
	if err := ensureRuntimeDirectory(paths.RuntimeDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.DaemonStderrLogFile()), 0o700); err != nil {
		return fmt.Errorf("Creating log directory: %w", err)
	}
	stderrLog, err := os.OpenFile(paths.DaemonStderrLogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("Opening daemon log: %w", err)
	}
	defer stderrLog.Close()

	cmd := exec.Command(InstalledBinary("forged"), "daemon")
	cmd.Stdout, cmd.Stderr = stderrLog, stderrLog
	// Its own session, so closing the terminal that started it does not stop the agent.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Starting daemon: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	info, err := platform.ProcessInfoForPID(cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("Recording daemon process: %w", err)
	}
	data, err := json.Marshal(info.Instance)
	if err != nil {
		return err
	}
	return platform.WriteFileDurable(spawnRecordPath(), data, 0o600)
}

func stopSpawnedDaemon() error {
	instance, running := spawnedDaemon()
	if running {
		if err := syscall.Kill(instance.PID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("Stopping daemon: %w", err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, alive := spawnedDaemon(); !alive {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("Stopping daemon: pid %d did not exit", instance.PID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err := os.Remove(spawnRecordPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Removing daemon record: %w", err)
	}
	return nil
}

func inspectSpawnedService() ServiceStatus {
	status := DefaultServiceStatus()
	binary := InstalledBinary("forged")
	if !binaryExecutable(binary) {
		status.Detail = "not installed"
		return status
	}
	status.Installed, status.ConfigValid, status.Loaded = true, true, true
	status.BinaryPath = binary
	status.PIDKnown = true
	status.Detail = "stopped (background process, no systemd)"
	if instance, running := spawnedDaemon(); running {
		status.Running, status.PID = true, instance.PID
		status.Detail = "running (background process, no systemd)"
	}
	return status
}
