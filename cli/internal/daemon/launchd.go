//go:build darwin

package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/itzzritik/forged/cli/internal/config"
)

const launchdLabel = "me.ritik.forged"

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ .Binary }}</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{ .LogFile }}</string>
    <key>StandardErrorPath</key>
    <string>{{ .LogFile }}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>FORGED_MASTER_PASSWORD</key>
        <string>{{ .MasterPassword }}</string>
    </dict>
</dict>
</plist>
`))

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
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

	f, err := os.Create(plist)
	if err != nil {
		return fmt.Errorf("creating plist: %w", err)
	}
	defer f.Close()

	data := struct {
		Label          string
		Binary         string
		LogFile        string
		MasterPassword string
	}{
		Label:          launchdLabel,
		Binary:         binaryPath,
		LogFile:        paths.LogFile(),
		MasterPassword: masterPassword,
	}

	if err := plistTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	return nil
}

func StartService() error {
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
	return launchctlRun(
		[]string{"kickstart", "-k", launchdServiceTarget()},
		nil,
	)
}

func StopService() error {
	return launchctlIgnore(
		[]string{"bootout", launchdServiceTarget()},
		[]string{"could not find service", "not found", "no such process"},
	)
}

func RestartService() error {
	StopService()
	return StartService()
}

func UninstallService() error {
	StopService()
	path := plistPath()
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}

func ServiceInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func launchdDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchdServiceTarget() string {
	return launchdDomain() + "/" + launchdLabel
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
