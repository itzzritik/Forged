//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/itzzritik/forged/cli/internal/config"
)

const serviceName = "forged"

var unitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description=Forged SSH Agent
After=default.target

[Service]
Type=simple
ExecStart={{ .Binary }} daemon
Restart=always
RestartSec=5
Environment=FORGED_MASTER_PASSWORD={{ .MasterPassword }}

[Install]
WantedBy=default.target
`))

func unitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", serviceName+".service")
}

func InstallService(paths config.Paths, masterPassword string) error {
	binaryPath, err := findBinary()
	if err != nil {
		return err
	}

	unitDir := filepath.Dir(unitPath())
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(unitPath())
	if err != nil {
		return fmt.Errorf("creating unit file: %w", err)
	}
	defer f.Close()

	data := struct {
		Binary         string
		MasterPassword string
	}{
		Binary:         binaryPath,
		MasterPassword: masterPassword,
	}

	if err := unitTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run()
	exec.Command("systemctl", "--user", "enable", serviceName).Run()

	return nil
}

func StartService() error {
	cmd := exec.Command("systemctl", "--user", "start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting service: %s: %w", string(out), err)
	}
	return nil
}

func StopService() error {
	cmd := exec.Command("systemctl", "--user", "stop", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stopping service: %s: %w", string(out), err)
	}
	return nil
}

func RestartService() error {
	cmd := exec.Command("systemctl", "--user", "restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restarting service: %s: %w", string(out), err)
	}
	return nil
}

func UninstallService() error {
	exec.Command("systemctl", "--user", "stop", serviceName).Run()
	exec.Command("systemctl", "--user", "disable", serviceName).Run()
	path := unitPath()
	if _, err := os.Stat(path); err == nil {
		os.Remove(path)
	}
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func ServiceInstalled() bool {
	_, err := os.Stat(unitPath())
	return err == nil
}

func InspectService(paths config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	if !ServiceInstalled() {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true
	status.ConfigValid = true

	cmd := exec.Command("systemctl", "--user", "show", serviceName, "--property=LoadState,ActiveState,SubState", "--value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		status.Detail = strings.TrimSpace(string(out))
		if status.Detail == "" {
			status.Detail = err.Error()
		}
		return status, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		status.Loaded = strings.TrimSpace(lines[0]) == "loaded"
	}
	if len(lines) > 1 {
		active := strings.TrimSpace(lines[1])
		status.Running = active == "active"
		if status.Detail == "" {
			status.Detail = active
		}
	}
	if len(lines) > 2 {
		sub := strings.TrimSpace(lines[2])
		if sub != "" {
			status.Detail = sub
		}
	}
	if status.Detail == "" {
		status.Detail = "installed"
	}

	return status, nil
}

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find forged binary: %w", err)
	}
	return filepath.Abs(self)
}
