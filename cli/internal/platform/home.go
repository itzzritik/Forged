package platform

import (
	"os"
	"os/user"
	"runtime"
)

// EnsureHome sets HOME from the user database when it is unset (cron, env -i), so every
// home-relative path and every child process (git, ssh) uses the real home, never the working directory.
func EnsureHome() {
	if runtime.GOOS == "windows" || os.Getenv("HOME") != "" {
		return
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		_ = os.Setenv("HOME", u.HomeDir)
	}
}
