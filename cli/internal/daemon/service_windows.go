//go:build windows

package daemon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
)

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

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
    </LogonTrigger>
  </Triggers>
  <Settings>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions>
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
    </Exec>
  </Actions>
</Task>`, xmlEscape(runtime.Binary), xmlEscape(strings.Join(runtime.Args, " ")))

	tmpFile := filepath.Join(os.TempDir(), "forged-task.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlBody), 0600); err != nil {
		return fmt.Errorf("Writing task XML: %w", err)
	}
	defer os.Remove(tmpFile)

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName(), "/XML", tmpFile, "/F")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Creating scheduled task (binary=%q): %w; schtasks output: %q",
			runtime.Binary, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}

func StartService() error {
	cmd := exec.Command("schtasks", "/Run", "/TN", taskName())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Starting task: %s: %w", string(out), err)
	}
	return nil
}

func StopService() error {
	cmd := exec.Command("schtasks", "/End", "/TN", taskName())
	cmd.CombinedOutput()
	return nil
}

func RestartService() error {
	StopService()
	return StartService()
}

func UninstallService() error {
	StopService()
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName(), "/F")
	cmd.CombinedOutput()
	return nil
}

func ServiceInstalled() bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName())
	return cmd.Run() == nil
}

func InspectService(paths config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	if !ServiceInstalled() {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true
	status.ConfigValid = true

	if binary, err := extractWindowsTaskBinary(taskName()); err == nil && binary != "" {
		status.BinaryPath = binary
		if !binaryExecutable(binary) {
			status.BinaryMissing = true
			status.ConfigValid = false
			status.Detail = fmt.Sprintf("service binary missing: %s", binary)
			return status, nil
		}
	}

	cmd := exec.Command("schtasks", "/Query", "/TN", taskName(), "/FO", "LIST")
	out, err := cmd.CombinedOutput()
	if err != nil {
		status.Detail = string(out)
		if status.Detail == "" {
			status.Detail = err.Error()
		}
		return status, nil
	}

	detail := strings.TrimSpace(string(out))
	status.Loaded = true
	status.Running = strings.Contains(detail, "Status: Running")
	if status.Running {
		status.Detail = "running"
	} else {
		status.Detail = "installed"
	}

	return status, nil
}

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
