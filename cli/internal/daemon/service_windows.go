//go:build windows

package daemon

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

const (
	windowsTaskStateUnknown  = 0
	windowsTaskStateDisabled = 1
	windowsTaskStateQueued   = 2
	windowsTaskStateReady    = 3
	windowsTaskStateRunning  = 4
)

// currentTaskUser returns "DOMAIN\\username" on Windows. Task Scheduler
// requires this in <UserId> for LogonTrigger and Principal blocks so the
// task installs without needing admin elevation.
func currentTaskUser() string {
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return u.Username
	}
	return ""
}

// taskName produces a per-user scheduled-task name so two users on the same
// Windows box don't clobber each other's installations. The username is part
// of the path produced by os.UserHomeDir(); we fall back to a generic name
// if for some reason the home dir is unavailable.
func taskName() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "ForgedSSHAgent"
	}
	user := filepath.Base(home)
	if user == "" || user == "." || user == string(filepath.Separator) {
		return "ForgedSSHAgent"
	}
	return "ForgedSSHAgent-" + user
}

func InstallService(paths config.Paths, runtime RuntimeSpec) error {
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return err
	}

	logDir := filepath.Dir(paths.LogFile())
	os.MkdirAll(logDir, 0700)

	userID := currentTaskUser()
	userBlock := ""
	principalRef := ""
	if userID != "" {
		userBlock = "\n      <UserId>" + xmlEscape(userID) + "</UserId>"
		principalRef = ` Context="Author"`
	}
	principalsBlock := ""
	if userID != "" {
		principalsBlock = fmt.Sprintf(`
  <Principals>
    <Principal id="Author">
      <UserId>%s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>`, xmlEscape(userID))
	}

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Forged SSH Agent daemon</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>%s
    </LogonTrigger>
  </Triggers>%s
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions%s>
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
    </Exec>
  </Actions>
</Task>`,
		userBlock,
		principalsBlock,
		principalRef,
		xmlEscape(runtime.Binary),
		xmlEscape(strings.Join(runtime.Args, " ")))

	tmp, err := os.CreateTemp("", "forged-task-*.xml")
	if err != nil {
		return windowsTaskXMLCreationError(paths, xmlBody, err)
	}
	tmpFile := tmp.Name()
	defer os.Remove(tmpFile)
	if _, err := tmp.WriteString(xmlBody); err != nil {
		tmp.Close()
		return fmt.Errorf("writing task XML: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing task XML: %w", err)
	}

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName(), "/XML", tmpFile, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Keep the XML on failure so the user can reproduce the schtasks call
		// manually and see the full error. Truncated TUI display would otherwise
		// hide schtasks's most useful diagnostics.
		diagPath, diagErr := writeWindowsTaskDiagnostic(paths, xmlBody)
		if diagErr != nil {
			return errors.Join(
				fmt.Errorf("creating scheduled task failed: %w; output: %q", err, strings.TrimSpace(string(out))),
				fmt.Errorf("saving task XML diagnostic: %w", diagErr),
			)
		}
		return fmt.Errorf("Creating scheduled task failed: %w; output: %q; xml saved to %s",
			err, strings.TrimSpace(string(out)), diagPath)
	}
	return nil
}

func windowsTaskXMLCreationError(paths config.Paths, xmlBody string, createErr error) error {
	diagPath, diagErr := writeWindowsTaskDiagnostic(paths, xmlBody)
	if diagErr != nil {
		return errors.Join(
			fmt.Errorf("creating temporary task XML: %w", createErr),
			fmt.Errorf("saving task XML diagnostic: %w", diagErr),
		)
	}
	return fmt.Errorf("creating temporary task XML: %w; xml saved to %s", createErr, diagPath)
}

func writeWindowsTaskDiagnostic(paths config.Paths, xmlBody string) (string, error) {
	diagPath := filepath.Join(paths.StateDir, "logs", "task-install-failed.xml")
	if err := os.MkdirAll(filepath.Dir(diagPath), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(diagPath, []byte(xmlBody), 0o600); err != nil {
		return "", err
	}
	return diagPath, nil
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}

func StartService() error {
	return runScheduledTaskCommand("start", "/Run", "/TN", taskName())
}

func StopService() error {
	state, installed, err := queryWindowsTaskState()
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}
	if !windowsTaskStateActive(state) {
		return nil
	}
	if err := runScheduledTaskCommand("stop", "/End", "/TN", taskName()); err != nil {
		state, installed, queryErr := queryWindowsTaskState()
		if queryErr == nil && (!installed || !windowsTaskStateActive(state)) {
			return nil
		}
		if queryErr != nil {
			return errors.Join(err, fmt.Errorf("rechecking scheduled task state: %w", queryErr))
		}
		return err
	}
	return waitForWindowsTaskStop(5 * time.Second)
}

func RestartService() error {
	if err := StopService(); err != nil {
		return err
	}
	return StartService()
}

func UninstallService() error {
	if err := StopService(); err != nil {
		return err
	}
	_, installed, err := queryWindowsTaskState()
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}
	if err := runScheduledTaskCommand("delete", "/Delete", "/TN", taskName(), "/F"); err != nil {
		_, installed, queryErr := queryWindowsTaskState()
		if queryErr == nil && !installed {
			return nil
		}
		if queryErr != nil {
			return errors.Join(err, fmt.Errorf("rechecking scheduled task state: %w", queryErr))
		}
		return err
	}
	return nil
}

func ServiceInstalled() (bool, error) {
	_, installed, err := queryWindowsTaskState()
	return installed, err
}

func InspectService(_ config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	state, installed, err := queryWindowsTaskState()
	if err != nil {
		return status, err
	}
	if !installed {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true
	status.ConfigValid = true

	binary, err := extractWindowsTaskBinary(taskName())
	if err != nil {
		status.ConfigValid = false
		status.Detail = fmt.Sprintf("reading scheduled task config: %v", err)
		return status, nil
	}
	if strings.TrimSpace(binary) == "" {
		status.ConfigValid = false
		status.Detail = "scheduled task has no executable"
		return status, nil
	}
	status.BinaryPath = binary
	if !binaryExecutable(binary) {
		status.BinaryMissing = true
		status.ConfigValid = false
		status.Detail = fmt.Sprintf("service binary missing: %s", binary)
		return status, nil
	}

	status.Loaded = true
	status.Running = state == windowsTaskStateRunning
	switch state {
	case windowsTaskStateUnknown:
		status.Detail = "unknown"
	case windowsTaskStateDisabled:
		status.Detail = "disabled"
	case windowsTaskStateQueued:
		status.Detail = "queued"
	case windowsTaskStateReady:
		status.Detail = "ready"
	case windowsTaskStateRunning:
		status.Detail = "running"
	}

	return status, nil
}

func queryWindowsTaskState() (int, bool, error) {
	powershell, err := windowsServicePowerShellPath()
	if err != nil {
		return 0, false, fmt.Errorf("finding PowerShell for scheduled task state: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", windowsTaskStateScript)
	cmd.Env = append(os.Environ(), "FORGED_TASK_NAME="+taskName())
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
			return 0, false, nil
		}
		message := strings.TrimSpace(string(out))
		if message == "" {
			return 0, false, fmt.Errorf("reading scheduled task state: %w", err)
		}
		return 0, false, fmt.Errorf("reading scheduled task state: %s: %w", message, err)
	}
	state, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false, fmt.Errorf("parsing scheduled task state %q: %w", strings.TrimSpace(string(out)), err)
	}
	if state < windowsTaskStateUnknown || state > windowsTaskStateRunning {
		return 0, false, fmt.Errorf("scheduled task returned unknown state %d", state)
	}
	return state, true, nil
}

func windowsTaskStateActive(state int) bool {
	return state == windowsTaskStateQueued || state == windowsTaskStateRunning
}

func waitForWindowsTaskStop(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		state, installed, err := queryWindowsTaskState()
		if err != nil {
			return err
		}
		if !installed || !windowsTaskStateActive(state) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("stopping scheduled task: timed out in state %d", state)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func runScheduledTaskCommand(action string, args ...string) error {
	out, err := exec.Command("schtasks", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		return fmt.Errorf("scheduled task %s failed: %w", action, err)
	}
	return fmt.Errorf("scheduled task %s failed: %s: %w", action, message, err)
}

func windowsServicePowerShellPath() (string, error) {
	for _, candidate := range []string{"powershell.exe", "pwsh.exe"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

const windowsTaskStateScript = `$ErrorActionPreference = 'Stop'
try {
  $service = New-Object -ComObject 'Schedule.Service'
  $service.Connect()
  $task = $service.GetFolder('\').GetTask($env:FORGED_TASK_NAME)
  [Console]::Out.Write([int]$task.State)
  exit 0
} catch {
  $exception = $_.Exception
  while ($null -ne $exception) {
    if ($exception.HResult -eq -2147024894) {
      exit 3
    }
    $exception = $exception.InnerException
  }
  [Console]::Error.Write($_.Exception.Message)
  exit 1
}`

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Cannot find Forged binary: %w", err)
	}
	return filepath.Abs(self)
}

// extractWindowsTaskBinary returns the executable declared in the Actions/Exec
// block of the registered scheduled task. Returns an empty string if the task
// has no Exec action.
func extractWindowsTaskBinary(name string) (string, error) {
	out, err := exec.Command("schtasks", "/Query", "/TN", name, "/XML").Output()
	if err != nil {
		return "", err
	}
	var task struct {
		Actions struct {
			Exec []struct {
				Command string `xml:"Command"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	if err := xml.Unmarshal(out, &task); err != nil {
		return "", err
	}
	for _, action := range task.Actions.Exec {
		if cmd := strings.TrimSpace(action.Command); cmd != "" {
			return cmd, nil
		}
	}
	return "", nil
}

func binaryExecutable(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return true
}
