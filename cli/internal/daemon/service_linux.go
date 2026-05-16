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

[Install]
WantedBy=default.target
`))

func unitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", serviceName+".service")
}

func InstallService(paths config.Paths, runtime RuntimeSpec) error {
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
		return fmt.Errorf("Creating unit file: %w", err)
	}
	defer f.Close()

	data := struct {
		ExecStart string
		Binary    string
	}{
		ExecStart: formatSystemdExecStart(runtime),
		Binary:    runtime.Binary,
	}

	if err := unitTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("Writing unit file: %w", err)
	}

	systemctlUser("daemon-reload").Run()
	systemctlUser("enable", serviceName).Run()

	return nil
}

func StartService() error {
	cmd := systemctlUser("start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Starting service: %s: %w", string(out), err)
	}
	return nil
}

func StopService() error {
	cmd := systemctlUser("stop", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Stopping service: %s: %w", string(out), err)
	}
	return nil
}

func RestartService() error {
	cmd := systemctlUser("restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Restarting service: %s: %w", string(out), err)
	}
	return nil
}

func UninstallService() error {
	systemctlUser("stop", serviceName).Run()
	systemctlUser("disable", serviceName).Run()
	path := unitPath()
	if _, err := os.Stat(path); err == nil {
		os.Remove(path)
	}
	systemctlUser("daemon-reload").Run()
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

	if binary, err := extractSystemdBinary(unitPath()); err == nil && binary != "" {
		status.BinaryPath = binary
		if !binaryExecutable(binary) {
			status.BinaryMissing = true
			status.ConfigValid = false
			status.Detail = fmt.Sprintf("service binary missing: %s", binary)
			return status, nil
		}
	}

	cmd := systemctlUser("show", serviceName, "--property=LoadState,ActiveState,SubState", "--value")
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
		return "", fmt.Errorf("Cannot find Forged binary: %w", err)
	}
	return filepath.Abs(self)
}

// extractSystemdBinary returns the binary path declared by the ExecStart= line
// in the given unit file. The path is the first quoted-or-unquoted token after
// the '='. Returns an empty string if the unit has no ExecStart line.
func extractSystemdBinary(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ExecStart=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "ExecStart="))
		value = strings.TrimLeft(value, "-@:!|+")
		if value == "" {
			return "", nil
		}
		if value[0] == '"' {
			if end := strings.Index(value[1:], "\""); end >= 0 {
				return value[1 : 1+end], nil
			}
		}
		if idx := strings.IndexAny(value, " \t"); idx > 0 {
			return value[:idx], nil
		}
		return value, nil
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
	return info.Mode()&0o111 != 0
}

func formatSystemdExecStart(runtime RuntimeSpec) string {
	parts := make([]string, 0, len(runtime.Args)+1)
	parts = append(parts, strconv.Quote(runtime.Binary))
	for _, arg := range runtime.Args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}

func systemctlUser(args ...string) *exec.Cmd {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Env = systemdUserEnv()
	return cmd
}

func systemdUserEnv() []string {
	env := os.Environ()
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
		env = append(env, "XDG_RUNTIME_DIR="+runtimeDir)
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		bus := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(bus); err == nil {
			env = append(env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+bus)
		}
	}
	return env
}
