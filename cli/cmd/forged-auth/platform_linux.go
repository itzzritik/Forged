//go:build linux

package main

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
)

func providerName() string { return "pkexec" }

func authorize(ctx context.Context, action sensitiveauth.Action) string {
	_ = action
	path, err := exec.LookPath("pkexec")
	if err != nil {
		return "unavailable"
	}

	cmd := exec.CommandContext(ctx, path, "--disable-internal-agent", "/bin/true")
	if err := cmd.Run(); err != nil {
		return "failed"
	}
	return "ok"
}

func startLockLoop(onLock func()) {
	if onLock == nil {
		return
	}
	go watchLinuxLocks(onLock)
}

func watchLinuxLocks(onLock func()) {
	gdbusPath, err := exec.LookPath("gdbus")
	if err != nil {
		return
	}

	for {
		cmd := exec.Command(
			gdbusPath,
			"monitor",
			"--session",
			"--dest", "org.freedesktop.ScreenSaver",
			"--object-path", "/org/freedesktop/ScreenSaver",
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			time.Sleep(time.Second)
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
		time.Sleep(time.Second)
	}
}
