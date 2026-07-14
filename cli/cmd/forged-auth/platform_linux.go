//go:build linux

package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
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

func hasGraphicalSession() bool {
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func startLockLoop(ctx context.Context, onLock func()) {
	if onLock == nil {
		return
	}
	watchLinuxLocks(ctx, onLock)
}

func watchLinuxLocks(ctx context.Context, onLock func()) {
	gdbusPath, err := exec.LookPath("gdbus")
	if err != nil {
		return
	}

	for {
		cmd := exec.CommandContext(
			ctx,
			gdbusPath,
			"monitor",
			"--session",
			"--dest", "org.freedesktop.ScreenSaver",
			"--object-path", "/org/freedesktop/ScreenSaver",
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			if !retryLockMonitor(ctx) {
				return
			}
			continue
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			if !retryLockMonitor(ctx) {
				return
			}
			continue
		}
		if err := cmd.Start(); err != nil {
			if !retryLockMonitor(ctx) {
				return
			}
			continue
		}

		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			line := strings.ToLower(scanner.Text())
			if strings.Contains(line, "activechanged") && strings.Contains(line, "true") {
				onLock()
			}
		}

		_ = cmd.Wait()
		if !retryLockMonitor(ctx) {
			return
		}
	}
}

func retryLockMonitor(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
