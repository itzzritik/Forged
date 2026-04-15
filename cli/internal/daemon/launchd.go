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

	"github.com/itzzritik/forged/cli/internal/config"
)

const launchdLabel = "me.ritik.forged.daemon"

var legacyLaunchdLabels = []string{"me.ritik.forged"}

type launchdTemplateData struct {
	Label          string
	Binary         string
	LogFile        string
	MasterPassword string
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
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{ xml .LogFile }}</string>
    <key>StandardErrorPath</key>
    <string>{{ xml .LogFile }}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>FORGED_MASTER_PASSWORD</key>
        <string>{{ xml .MasterPassword }}</string>
    </dict>
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

func InstallService(paths config.Paths, masterPassword string) error {
	binaryPath, err := findBinary()
	if err != nil {
		return err
	}

	logDir := filepath.Dir(paths.LogFile())
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	plist := plistPath()
	if err := os.MkdirAll(filepath.Dir(plist), 0755); err != nil {
		return err
	}

	data := launchdTemplateData{
		Label:          launchdLabel,
		Binary:         binaryPath,
		LogFile:        paths.LogFile(),
		MasterPassword: masterPassword,
	}

	raw, err := renderLaunchdPlist(data)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(plist), launchdLabel+".*.plist")
	if err != nil {
		return fmt.Errorf("creating plist: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("writing plist: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing plist: %w", err)
	}

	if err := validateLaunchdPlist(tmp.Name()); err != nil {
		return err
	}

	if err := os.Rename(tmp.Name(), plist); err != nil {
		return fmt.Errorf("installing plist: %w", err)
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
	if err := launchctlRun(
		[]string{"bootstrap", launchdDomain(), plistPath()},
		nil,
	); err != nil {
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

func findBinary() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find forged binary: %w", err)
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

		password, err := loadLaunchdMasterPassword(path)
		if err != nil {
			return fmt.Errorf("reading legacy launchd service %s: %w", path, err)
		}
		if err := InstallService(paths, password); err != nil {
			return fmt.Errorf("installing migrated launchd service: %w", err)
		}
		return nil
	}

	return nil
}

func loadLaunchdMasterPassword(path string) (string, error) {
	out, err := exec.Command("plutil", "-extract", "EnvironmentVariables.FORGED_MASTER_PASSWORD", "raw", "-o", "-", path).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}

	password := strings.TrimSuffix(string(out), "\n")
	if password == "" {
		return "", fmt.Errorf("FORGED_MASTER_PASSWORD is empty")
	}
	return password, nil
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
		return nil, fmt.Errorf("rendering plist: %w", err)
	}
	return buf.Bytes(), nil
}

func validateLaunchdPlist(path string) error {
	cmd := exec.Command("plutil", "-lint", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("validating plist: %s: %w", strings.TrimSpace(string(out)), err)
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
