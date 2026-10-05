//go:build windows

package platform

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetConsoleProcessList = modkernel32.NewProc("GetConsoleProcessList")
	procFreeConsole           = modkernel32.NewProc("FreeConsole")
)

// Task Scheduler gives the daemon its own visible console; closing it would
// stop the agent.
func DetachOwnedConsole() {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n == 1 {
		_, _, _ = procFreeConsole.Call()
	}
}

func HideChildConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
