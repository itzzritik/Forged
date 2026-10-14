package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// SSHRoutingPlatform reports whether this OS can scope SSH key access per route.
func SSHRoutingPlatform() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	default:
		return false
	}
}

// SSHRoutingSupported reports whether route-scoped SSH key access is safe on this platform.
func SSHRoutingSupported() bool {
	return SSHRoutingPlatform() && routingShellSupported() && (runtime.GOOS != "linux" || linuxSSHClientsSupportRouting())
}

// OpenSSH runs Match exec hooks as `$SHELL -c`, and the hooks `exec` the helper so its parent is ssh
// itself. A shell without that (PowerShell, nushell) would fail every hook, leaving ssh no keys at all.
func routingShellSupported() bool {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return true
	}
	switch filepath.Base(shell) {
	case "sh", "bash", "zsh", "dash", "ksh", "ksh93", "mksh", "oksh", "ash", "yash", "fish", "csh", "tcsh":
		return true
	}
	return false
}

// Route hooks need `Match final`; OpenSSH before 8.0 rejects the whole config, breaking every ssh.
var linuxSSHClientsSupportRouting = sync.OnceValue(func() bool {
	clients := map[string]struct{}{}
	if _, err := os.Stat("/usr/bin/ssh"); err == nil {
		clients["/usr/bin/ssh"] = struct{}{}
	}
	if path, err := exec.LookPath("ssh"); err == nil {
		clients[path] = struct{}{}
	}
	for path := range clients {
		if !openSSHAtLeast8(path) {
			return false
		}
	}
	return len(clients) > 0
})

var openSSHMajorPattern = regexp.MustCompile(`OpenSSH_(\d+)\.`)

func openSSHAtLeast8(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, path, "-V").CombinedOutput()
	match := openSSHMajorPattern.FindSubmatch(out)
	if match == nil {
		return false
	}
	major, err := strconv.Atoi(string(match[1]))
	return err == nil && major >= 8
}
