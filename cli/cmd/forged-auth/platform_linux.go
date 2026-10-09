//go:build linux

package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

func providerName() string { return "pkexec" }

func authorize(ctx context.Context, action sensitiveauth.Action) string {
	_ = action
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if !hasGraphicalSession() {
		return "unavailable_by_environment"
	}
	path, err := exec.LookPath("pkexec")
	if err != nil {
		return "unavailable_by_environment"
	}

	cmd := exec.CommandContext(ctx, path, "--disable-internal-agent", "/bin/true")
	if err := cmd.Run(); err != nil {
		if parent.Err() != nil {
			return "canceled"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "failed"
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 126:
				return "canceled"
			case 127:
				return "failed"
			}
		}
		return "failed"
	}
	return "ok"
}

func status(context.Context) string {
	if !hasGraphicalSession() {
		return "unavailable_by_environment"
	}
	if _, err := exec.LookPath("pkexec"); err != nil {
		return "unavailable_by_environment"
	}
	return "ok"
}

func collectPassword(context.Context, string) (string, string) {
	return "unavailable_by_platform", ""
}

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func startLockLoop(ctx context.Context, onLock func()) {
	if onLock == nil {
		return
	}
	var watchers sync.WaitGroup
	for _, monitor := range []struct {
		args  []string
		match func(string) bool
	}{
		{
			args: []string{"--session", "--dest", "org.freedesktop.ScreenSaver", "--object-path", "/org/freedesktop/ScreenSaver"},
			match: func(line string) bool {
				return strings.Contains(line, "activechanged") && strings.Contains(line, "true")
			},
		},
		{
			args: []string{"--system", "--dest", "org.freedesktop.login1", "--object-path", "/org/freedesktop/login1"},
			match: func(line string) bool {
				return strings.Contains(line, "prepareforsleep") && strings.Contains(line, "true")
			},
		},
	} {
		if monitor.args[0] == "--session" && !hasSessionBus() {
			continue
		}
		watchers.Add(1)
		go func(args []string, match func(string) bool) {
			defer watchers.Done()
			watchLinuxMonitor(ctx, onLock, args, match)
		}(monitor.args, monitor.match)
	}
	watchers.Wait()
}

func watchLinuxMonitor(ctx context.Context, onLock func(), args []string, match func(string) bool) {
	gdbusPath, err := exec.LookPath("gdbus")
	if err != nil {
		return
	}

	delay := time.Second
	for {
		started := time.Now()
		runLinuxMonitor(ctx, gdbusPath, args, onLock, match)
		if time.Since(started) >= maxMonitorBackoff {
			delay = time.Second
		}
		if !retryLockMonitor(ctx, delay) {
			return
		}
		delay = min(2*delay, maxMonitorBackoff)
	}
}

const maxMonitorBackoff = time.Minute

func runLinuxMonitor(ctx context.Context, gdbusPath string, args []string, onLock func(), match func(string) bool) {
	cmd := exec.CommandContext(ctx, gdbusPath, append([]string{"monitor"}, args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		if match(strings.ToLower(scanner.Text())) {
			onLock()
		}
	}
	_ = cmd.Wait()
}

func hasSessionBus() bool {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "" {
		return true
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "bus"))
	return err == nil
}

func retryLockMonitor(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
