//go:build linux

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
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
	if err := InstallBinaries(runtime.Binary); err != nil {
		return fmt.Errorf("Installing Forged binaries: %w", err)
	}
	if !InInstallDir(runtime.Binary) {
		runtime.Binary = InstalledBinary("forged")
	}

	unitDir := filepath.Dir(unitPath())
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return err
	}

	f, err := os.CreateTemp(unitDir, "."+serviceName+".*.service")
	if err != nil {
		return fmt.Errorf("Creating temporary unit file: %w", err)
	}
	defer os.Remove(f.Name())

	data := struct {
		ExecStart string
		Binary    string
	}{
		ExecStart: formatSystemdExecStart(runtime),
		Binary:    runtime.Binary,
	}

	if err := unitTemplate.Execute(f, data); err != nil {
		f.Close()
		return fmt.Errorf("Writing unit file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("Closing unit file: %w", err)
	}
	if err := os.Rename(f.Name(), unitPath()); err != nil {
		return fmt.Errorf("Installing unit file: %w", err)
	}

	if out, err := systemctlUser("daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("Reloading systemd user services: %s: %w", string(out), err)
	}
	if out, err := systemctlUser("enable", serviceName).CombinedOutput(); err != nil {
		return fmt.Errorf("Enabling service: %s: %w", string(out), err)
	}
	EnsureLinger()

	return nil
}

func InspectLinger() LingerState {
	if platform.HasDisplay() && !platform.OverSSH() {
		return LingerState{}
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return LingerState{}
	}
	u, err := user.Current()
	if err != nil {
		return LingerState{}
	}
	_, err = os.Stat(filepath.Join("/var/lib/systemd/linger", u.Username))
	return LingerState{Applies: true, On: err == nil, User: u.Username}
}

func EnsureLinger() LingerState {
	state := InspectLinger()
	if !state.Applies || state.On {
		return state
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "loginctl", "--no-ask-password", "enable-linger").Run()
	return InspectLinger()
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
	_ = os.RemoveAll(filepath.Join(InstallDir(), "bin"))
	return nil
}

func ServiceInstalled() (bool, error) {
	_, err := os.Stat(unitPath())
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking systemd service: %w", err)
}

func InspectService(paths config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	installed, err := ServiceInstalled()
	if err != nil {
		return status, err
	}
	if !installed {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true
	status.ConfigValid = true

	binary, args, err := extractSystemdCommand(unitPath())
	if err != nil {
		invalidateServiceConfig(&status, fmt.Sprintf("reading service command: %v", err))
	} else {
		status.BinaryPath = binary
		if err := validateDaemonServiceCommand(binary, args); err != nil {
			invalidateServiceConfig(&status, fmt.Sprintf("invalid service command: %v", err))
		} else if !binaryExecutable(binary) {
			status.BinaryMissing = true
			invalidateServiceConfig(&status, fmt.Sprintf("service binary missing: %s", binary))
		}
	}

	cmd := systemctlUser("show", serviceName, "--property=LoadState,ActiveState,SubState,MainPID", "--value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		setServiceDetail(&status, detail)
		return status, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		status.Loaded = strings.TrimSpace(lines[0]) == "loaded"
	}
	if len(lines) > 1 {
		active := strings.TrimSpace(lines[1])
		status.Running = active == "active"
		setServiceDetail(&status, active)
	}
	if len(lines) > 2 {
		sub := strings.TrimSpace(lines[2])
		if sub != "" {
			if status.ConfigValid {
				status.Detail = sub
			} else {
				setServiceDetail(&status, sub)
			}
		}
	}
	if len(lines) > 3 {
		if pid, err := strconv.Atoi(strings.TrimSpace(lines[3])); err == nil && pid >= 0 {
			status.PID = pid
			status.PIDKnown = true
		}
	}
	setServiceDetail(&status, "installed")

	return status, nil
}

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Cannot find Forged binary: %w", err)
	}
	return filepath.Abs(self)
}

func extractSystemdCommand(path string) (string, []string, error) {
	command := ""
	if err := readSystemdCommand(path, &command); err != nil {
		return "", nil, err
	}
	// Read drop-ins from disk so stale manager cache cannot hide an override.
	dropIns, err := os.ReadDir(path + ".d")
	if err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	for _, dropIn := range dropIns {
		if dropIn.IsDir() || !strings.HasSuffix(dropIn.Name(), ".conf") {
			continue
		}
		if err := readSystemdCommand(filepath.Join(path+".d", dropIn.Name()), &command); err != nil {
			return "", nil, err
		}
	}
	if command == "" {
		return "", nil, fmt.Errorf("missing ExecStart")
	}
	return parseSystemdCommand(command)
}

func readSystemdCommand(path string, command *string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	inService := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inService = trimmed == "[Service]"
			continue
		}
		key, rawValue, found := strings.Cut(trimmed, "=")
		if !inService || !found || strings.TrimSpace(key) != "ExecStart" {
			continue
		}
		raw := strings.TrimSpace(rawValue)
		if raw == "" {
			*command = ""
			continue
		}
		value := strings.TrimLeft(raw, "-@:!|+")
		if value == "" {
			return fmt.Errorf("%s: empty ExecStart command", path)
		}
		if *command != "" {
			return fmt.Errorf("%s: multiple ExecStart commands", path)
		}
		*command = value
	}
	return nil
}

func parseSystemdCommand(value string) (string, []string, error) {
	var command []string
	for value = strings.TrimSpace(value); value != ""; value = strings.TrimSpace(value) {
		operand, rest, err := parseSystemdOperand(value)
		if err != nil {
			return "", nil, err
		}
		command = append(command, strings.ReplaceAll(operand, "%%", "%"))
		value = rest
	}
	if len(command) == 0 {
		return "", nil, fmt.Errorf("empty ExecStart")
	}
	return command[0], command[1:], nil
}

func parseSystemdOperand(value string) (string, string, error) {
	if value[0] == '\'' {
		end := strings.IndexByte(value[1:], '\'')
		if end < 0 {
			return "", "", fmt.Errorf("invalid single-quoted ExecStart operand")
		}
		rest := value[end+2:]
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			return "", "", fmt.Errorf("missing whitespace after quoted ExecStart operand")
		}
		return value[1 : end+1], rest, nil
	}
	if value[0] != '"' {
		if end := strings.IndexAny(value, " \t"); end >= 0 {
			return value[:end], value[end:], nil
		}
		return value, "", nil
	}
	quoted, err := strconv.QuotedPrefix(value)
	if err != nil {
		return "", "", fmt.Errorf("invalid quoted ExecStart operand: %w", err)
	}
	operand, err := strconv.Unquote(quoted)
	if err != nil {
		return "", "", fmt.Errorf("unquoting ExecStart operand: %w", err)
	}
	rest := value[len(quoted):]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", "", fmt.Errorf("missing whitespace after quoted ExecStart operand")
	}
	return operand, rest, nil
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
	return strings.ReplaceAll(strings.Join(parts, " "), "%", "%%")
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
