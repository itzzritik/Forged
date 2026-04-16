//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
ExecStart={{ .ExecStart }}
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

func InstallService(paths config.Paths, masterPassword string, runtime RuntimeSpec) error {
	runtime, err := normalizeRuntimeSpec(runtime)
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
		ExecStart      string
		Binary         string
		MasterPassword string
	}{
		ExecStart:      formatSystemdExecStart(runtime),
		Binary:         runtime.Binary,
		MasterPassword: masterPassword,
	}

	if err := unitTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run()
	exec.Command("systemctl", "--user", "enable", serviceName).Run()

	return nil
}

func ReadInstalledServicePassword() (string, error) {
	data, err := os.ReadFile(unitPath())
	if err != nil {
		return "", err
	}

	const prefix = "Environment=FORGED_MASTER_PASSWORD="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimPrefix(strings.TrimSpace(line), prefix), nil
		}
	}

	return "", fmt.Errorf("installed service password not found")
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

func formatSystemdExecStart(runtime RuntimeSpec) string {
	parts := make([]string, 0, len(runtime.Args)+1)
	parts = append(parts, strconv.Quote(runtime.Binary))
	for _, arg := range runtime.Args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}
