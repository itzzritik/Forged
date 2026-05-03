//go:build !windows

package actions

import "syscall"

func detachedDaemonProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
