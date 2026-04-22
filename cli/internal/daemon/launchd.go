//go:build darwin

package daemon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

const launchdLabel = "me.ritik.forged.daemon"

var legacyLaunchdLabels = []string{"me.ritik.forged"}

type launchdTemplateData struct {
	Label   string
	Binary  string
	Args    []string
	LogFile string
}

type launchdServiceFile struct {
	Label  string
	Path   string
	Legacy bool
}

var plistTemplate = template.Must(template.New("plist").Funcs(template.FuncMap{
	"xml": xmlEscape,
}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ xml .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ xml .Binary }}</string>
{{- range .Args }}
        <string>{{ xml . }}</string>
{{- end }}
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{ xml .LogFile }}</string>
    <key>StandardErrorPath</key>
    <string>{{ xml .LogFile }}</string>
</dict>
</plist>
`))

func plistPath() string {
	return plistPathForLabel(launchdLabel)
}

func plistPathForLabel(label string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func legacyPlistPaths() []string {
	paths := make([]string, 0, len(legacyLaunchdLabels))
	for _, label := range legacyLaunchdLabels {
		paths = append(paths, plistPathForLabel(label))
	}
	return paths
}

func InstallService(paths config.Paths, runtime RuntimeSpec) error {
	runtime, err := normalizeRuntimeSpec(runtime)
	if err != nil {
		return err
	}

	logDir := filepath.Dir(paths.LogFile())
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("Creating log directory: %w", err)
	}

	plist := plistPath()
	if err := os.MkdirAll(filepath.Dir(plist), 0755); err != nil {
		return err
	}

	data := launchdTemplateData{
		Label:   launchdLabel,
		Binary:  runtime.Binary,
		Args:    runtime.Args,
		LogFile: paths.LogFile(),
	}

	raw, err := renderLaunchdPlist(data)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(plist), launchdLabel+".*.plist")
	if err != nil {
		return fmt.Errorf("Creating plist: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("Writing plist: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing plist: %w", err)
	}

	if err := validateLaunchdPlist(tmp.Name()); err != nil {
		return err
	}

	if err := os.Rename(tmp.Name(), plist); err != nil {
		return fmt.Errorf("Installing plist: %w", err)
	}

	return nil
}

func StartService() error {
	paths := config.DefaultPaths()
	if err := migrateLegacyLaunchdService(paths); err != nil {
		return err
	}
	if err := launchctlIgnore(
		[]string{"bootout", launchdServiceTarget()},
		[]string{"could not find service", "service is disabled", "not found", "no such process"},
	); err != nil {
		return err
	}
	waitForDaemonExit(paths)
	if err := bootstrapLaunchdService(); err != nil {
		return err
	}
	if err := launchctlRun(
		[]string{"enable", launchdServiceTarget()},
		[]string{"already enabled"},
	); err != nil {
		return err
	}
	if err := launchctlRun(
		[]string{"kickstart", "-k", launchdServiceTarget()},
		nil,
	); err != nil {
		return err
	}

	_ = removeLegacyLaunchdPlists()
	return nil
}

func bootstrapLaunchdService() error {
	args := []string{"bootstrap", launchdDomain(), plistPath()}
	err := launchctlRun(args, nil)
	if err == nil {
		return nil
	}
	if !isIgnorableLaunchdError(err.Error(), []string{"bootstrap failed: 5: input/output error"}) {
		return err
	}

	time.Sleep(300 * time.Millisecond)
	return launchctlRun(
		args,
		[]string{"service already loaded", "already loaded", "already bootstrapped"},
	)
}

func StopService() error {
	var firstErr error

	for _, service := range existingLaunchdServiceFiles() {
		ignorable := []string{"could not find service", "not found", "no such process"}
		if service.Legacy {
			ignorable = append(ignorable, "input/output error")
		}
		if err := launchctlIgnore(
			[]string{"bootout", launchdServiceTargetForLabel(service.Label)},
			ignorable,
		); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func RestartService() error {
	StopService()
	return StartService()
}

func UninstallService() error {
	StopService()
	for _, service := range existingLaunchdServiceFiles() {
		if err := os.Remove(service.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func ServiceInstalled() bool {
	return len(existingLaunchdServiceFiles()) > 0
}

func InspectService(paths config.Paths) (ServiceStatus, error) {
	status := DefaultServiceStatus()
	services := existingLaunchdServiceFiles()
	if len(services) == 0 {
		status.Detail = "not installed"
		return status, nil
	}

	status.Installed = true

	for _, service := range services {
		if err := validateLaunchdPlist(service.Path); err != nil {
			status.Detail = err.Error()
			return status, nil
		}
	}
	status.ConfigValid = true

	for _, service := range services {
		out, err := exec.Command("launchctl", "print", launchdServiceTargetForLabel(service.Label)).CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(out))
			if message == "" {
				message = err.Error()
			}
			lower := strings.ToLower(message)
			if strings.Contains(lower, "could not find service") || strings.Contains(lower, "not found") {
				continue
			}
			status.Detail = message
			return status, nil
		}

		status.Loaded = true
		status.Running = launchdPrintIndicatesRunning(string(out))
		switch {
		case service.Legacy && status.Running:
			status.Detail = "legacy service running"
		case service.Legacy:
			status.Detail = "legacy service loaded but not running"
		case status.Running:
			status.Detail = "running"
		default:
			status.Detail = "loaded but not running"
		}
		return status, nil
	}

	if services[0].Legacy {
		status.Detail = "legacy service installed; will migrate on next start"
	} else {
		status.Detail = "installed but not loaded"
	}

	return status, nil
}

func launchdDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchdServiceTarget() string {
	return launchdServiceTargetForLabel(launchdLabel)
}

func launchdServiceTargetForLabel(label string) string {
	return launchdDomain() + "/" + label
}

func waitForDaemonExit(paths config.Paths) {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, running := IsRunning(paths); !running {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Cannot find Forged binary: %w", err)
	}
	abs, err := filepath.Abs(self)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}

func launchctlRun(args []string, ignorable []string) error {
	cmd := exec.Command("launchctl", args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(stderr.String())
	if isIgnorableLaunchdError(message, ignorable) {
		return nil
	}
	if message == "" {
		return err
	}
	return fmt.Errorf("%s", message)
}

func launchctlIgnore(args []string, ignorable []string) error {
	return launchctlRun(args, ignorable)
}

func isIgnorableLaunchdError(message string, ignorable []string) bool {
	lower := strings.ToLower(message)
	for _, needle := range ignorable {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func existingLaunchdServiceFiles() []launchdServiceFile {
	var services []launchdServiceFile

	current := plistPath()
	if _, err := os.Stat(current); err == nil {
		services = append(services, launchdServiceFile{
			Label: launchdLabel,
			Path:  current,
		})
	}

	for _, label := range legacyLaunchdLabels {
		path := plistPathForLabel(label)
		if _, err := os.Stat(path); err == nil {
			services = append(services, launchdServiceFile{
				Label:  label,
				Path:   path,
				Legacy: true,
			})
		}
	}

	return services
}

func migrateLegacyLaunchdService(paths config.Paths) error {
	if _, err := os.Stat(plistPath()); err == nil {
		return nil
	}

	for _, path := range legacyPlistPaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		runtime, err := DefaultRuntimeSpec()
		if err != nil {
			return err
		}
		if err := InstallService(paths, runtime); err != nil {
			return fmt.Errorf("Installing migrated launchd service: %w", err)
		}
		return nil
	}

	return nil
}

func removeLegacyLaunchdPlists() error {
	for _, path := range legacyPlistPaths() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func renderLaunchdPlist(data launchdTemplateData) ([]byte, error) {
	var buf bytes.Buffer
	if err := plistTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("Rendering plist: %w", err)
	}
	return buf.Bytes(), nil
}

func validateLaunchdPlist(path string) error {
	cmd := exec.Command("plutil", "-lint", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Validating plist: %s: %w", strings.TrimSpace(string(out)), err)
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

func launchdPrintIndicatesRunning(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "pid =") && !strings.Contains(lower, "pid = 0")
}
