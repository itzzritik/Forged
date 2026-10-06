//go:build windows

package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
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
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}
	// Held through registration so a concurrent install can't prune our copy.
	root := stagedBinariesRoot(paths)
	unlock, err := lockStagedBinaries(root)
	if err != nil {
		return fmt.Errorf("locking staged daemon binaries: %w", err)
	}
	defer unlock()
	staged, err := stageDaemonBinaries(root, runtime.Binary)
	if err != nil {
		return fmt.Errorf("staging daemon binary: %w", err)
	}
	if err := InstallBinaries(runtime.Binary); err != nil {
		return fmt.Errorf("installing Forged binaries: %w", err)
	}
	runtime.Binary = staged

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

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
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
	if _, err := tmp.Write(taskXML(xmlBody)); err != nil {
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
	pruneStagedDaemonBinaries(root, filepath.Dir(staged))
	return nil
}

// Windows can't replace a running exe, so the task runs a content-addressed
// copy; the auth helper sits beside it because the daemon looks there.
const (
	stagedDaemon = "forged.exe"
	stagedHelper = "forged-auth.exe"
)

func stagedBinariesRoot(paths config.Paths) string {
	if root := InstallDir(); root != "" {
		return filepath.Join(root, "daemon")
	}
	return filepath.Join(paths.ConfigDir, "bin")
}

func stageDaemonBinaries(root, binary string) (string, error) {
	if rel, err := filepath.Rel(root, binary); err == nil && !strings.HasPrefix(rel, "..") {
		return binary, nil
	}
	daemon, err := os.ReadFile(binary)
	if err != nil {
		return "", err
	}
	helper, err := os.ReadFile(filepath.Join(filepath.Dir(binary), stagedHelper))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Both files name the copy, so a changed helper never targets a running one.
	digest := sha256.New()
	fmt.Fprintf(digest, "%d:", len(daemon))
	digest.Write(daemon)
	digest.Write(helper)
	dir := filepath.Join(root, hex.EncodeToString(digest.Sum(nil)[:8]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := writeStagedFile(filepath.Join(dir, stagedDaemon), daemon); err != nil {
		return "", err
	}
	if helper != nil {
		if err := writeStagedFile(filepath.Join(dir, stagedHelper), helper); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, stagedDaemon), nil
}

func lockStagedBinaries(root string) (func(), error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(root, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := platform.LockFileWait(file); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = platform.UnlockFile(file)
		_ = file.Close()
	}, nil
}

// Copies are only ever completed by rename, so an existing one is whole.
func writeStagedFile(target string, data []byte) error {
	if info, err := os.Stat(target); err == nil && info.Size() == int64(len(data)) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".stage-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// A copy a running daemon still holds fails to delete and is retried later.
func pruneStagedDaemonBinaries(root, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		dir := filepath.Join(root, entry.Name())
		if entry.IsDir() && !strings.EqualFold(dir, keep) {
			_ = os.RemoveAll(dir)
		}
	}
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
	if err := os.WriteFile(diagPath, taskXML(xmlBody), 0o600); err != nil {
		return "", err
	}
	return diagPath, nil
}

// schtasks rejects UTF-8 with an XML declaration; UTF-16LE+BOM also keeps
// non-ASCII paths intact.
func taskXML(body string) []byte {
	return append([]byte{0xFF, 0xFE}, platform.UTF16LE(body)...)
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
	if installed {
		if err := runScheduledTaskCommand("delete", "/Delete", "/TN", taskName(), "/F"); err != nil {
			_, installed, queryErr := queryWindowsTaskState()
			if queryErr != nil {
				return errors.Join(err, fmt.Errorf("rechecking scheduled task state: %w", queryErr))
			}
			if installed {
				return err
			}
		}
	}
	_ = os.RemoveAll(stagedBinariesRoot(config.DefaultPaths()))
	_ = os.RemoveAll(filepath.Join(InstallDir(), "bin"))
	return nil
}

func ServiceInstalled() (bool, error) {
	_, installed, err := queryWindowsTask()
	return installed, err
}

func InspectService(_ config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	task, installed, err := queryWindowsTask()
	if err != nil {
		return status, err
	}
	if !installed {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true
	status.ConfigValid = true

	binary, args, err := task.command()
	if err != nil {
		invalidateServiceConfig(&status, fmt.Sprintf("reading scheduled task config: %v", err))
		return status, nil
	}
	if err := validateDaemonServiceCommand(binary, args); err != nil {
		invalidateServiceConfig(&status, fmt.Sprintf("invalid scheduled task command: %v", err))
		return status, nil
	}
	status.BinaryPath = binary
	if !binaryExecutable(binary) {
		status.BinaryMissing = true
		invalidateServiceConfig(&status, fmt.Sprintf("service binary missing: %s", binary))
		return status, nil
	}

	status.Loaded = true
	status.Running = task.State == windowsTaskStateRunning
	if status.Running && len(task.PIDs) == 1 && task.PIDs[0] > 0 {
		status.PID = task.PIDs[0]
		status.PIDKnown = true
	}
	switch task.State {
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

const windowsTaskActionExec = 0

type windowsTask struct {
	State   int                 `json:"state"`
	Actions []windowsTaskAction `json:"actions"`
	PIDs    []int               `json:"pids"`
}

type windowsTaskAction struct {
	Type      int    `json:"type"`
	Path      string `json:"path"`
	Arguments string `json:"arguments"`
}

func (t windowsTask) command() (string, []string, error) {
	if len(t.Actions) != 1 || t.Actions[0].Type != windowsTaskActionExec {
		return "", nil, fmt.Errorf("expected one Exec action")
	}
	action := t.Actions[0]
	return strings.Trim(strings.TrimSpace(action.Path), `"`), strings.Fields(action.Arguments), nil
}

func queryWindowsTaskState() (int, bool, error) {
	task, installed, err := queryWindowsTask()
	return task.State, installed, err
}

// COM, not `schtasks /Query`: its output is localized and mislabeled UTF-16.
func queryWindowsTask() (windowsTask, bool, error) {
	powershell, err := windowsServicePowerShellPath()
	if err != nil {
		return windowsTask{}, false, fmt.Errorf("finding PowerShell for scheduled task state: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", windowsTaskQueryScript)
	cmd.Env = append(os.Environ(), "FORGED_TASK_NAME="+taskName())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
			return windowsTask{}, false, nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return windowsTask{}, false, fmt.Errorf("reading scheduled task state: %w", err)
		}
		return windowsTask{}, false, fmt.Errorf("reading scheduled task state: %s: %w", message, err)
	}
	var task windowsTask
	if err := json.Unmarshal(bytes.TrimSpace(out), &task); err != nil {
		return windowsTask{}, false, fmt.Errorf("parsing scheduled task state %q: %w", strings.TrimSpace(string(out)), err)
	}
	if task.State < windowsTaskStateUnknown || task.State > windowsTaskStateRunning {
		return windowsTask{}, false, fmt.Errorf("scheduled task returned unknown state %d", task.State)
	}
	return task, true, nil
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

// \u escapes keep the JSON independent of the console code page.
const windowsTaskQueryScript = `$ErrorActionPreference = 'Stop'
try {
  $service = New-Object -ComObject 'Schedule.Service'
  $service.Connect()
  $task = $service.GetFolder('\').GetTask($env:FORGED_TASK_NAME)
  $actions = @(foreach ($action in $task.Definition.Actions) {
    if ($action.Type -eq 0) {
      @{ type = 0; path = [string]$action.Path; arguments = [string]$action.Arguments }
    } else {
      @{ type = [int]$action.Type }
    }
  })
  $pids = @(foreach ($instance in $task.GetInstances(0)) { [int]$instance.EnginePID })
  $json = ConvertTo-Json -Compress -Depth 4 -InputObject @{ state = [int]$task.State; actions = $actions; pids = $pids }
  $json = [regex]::Replace($json, '[^\x00-\x7F]', { param($m) '\u{0:x4}' -f [int][char]$m.Value })
  [Console]::Out.Write($json)
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
