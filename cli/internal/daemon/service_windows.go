//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/itzzritik/forged/cli/internal/config"
)

const taskName = "ForgedSSHAgent"

func InstallService(paths config.Paths, masterPassword string) error {
	binaryPath, err := findBinary()
	if err != nil {
		return err
	}

	logDir := filepath.Dir(paths.LogFile())
	os.MkdirAll(logDir, 0700)

	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
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
      <Arguments>daemon</Arguments>
    </Exec>
  </Actions>
</Task>`, binaryPath)

	tmpFile := filepath.Join(os.TempDir(), "forged-task.xml")
	if err := os.WriteFile(tmpFile, []byte(xml), 0600); err != nil {
		return fmt.Errorf("writing task xml: %w", err)
	}
	defer os.Remove(tmpFile)

	os.Setenv("FORGED_MASTER_PASSWORD", masterPassword)

	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/XML", tmpFile, "/F")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating scheduled task: %s: %w", string(out), err)
	}

	return nil
}

func StartService() error {
	cmd := exec.Command("schtasks", "/Run", "/TN", taskName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting task: %s: %w", string(out), err)
	}
	return nil
}

func StopService() error {
	cmd := exec.Command("schtasks", "/End", "/TN", taskName)
	cmd.CombinedOutput()
	return nil
}

func RestartService() error {
	StopService()
	return StartService()
}

func UninstallService() error {
	StopService()
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	cmd.CombinedOutput()
	return nil
}

func ServiceInstalled() bool {
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	return cmd.Run() == nil
}

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find forged binary: %w", err)
	}
	return filepath.Abs(self)
}
