//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// rootInUserHome reports a root process whose home belongs to someone else (sudo keeping HOME):
// everything it wrote there would be root-owned, and OpenSSH ignores a ~/.ssh/config it does not own.
func rootInUserHome() bool {
	if os.Geteuid() != 0 {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(home)
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid != 0
}
