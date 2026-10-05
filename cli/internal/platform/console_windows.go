//go:build windows

package platform

import (
	"encoding/binary"
	"os"
	"os/exec"
	"regexp"
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

// mintty without ConPTY hands console programs these pipes instead of a
// console; current MSYS runtimes append "-nat" for native programs.
var msysPTYPipe = regexp.MustCompile(`^\\(?:Device\\NamedPipe\\)?(?:cygwin|msys)-[^-]+-pty\d+-(?:from|to)-master(?:-\w+)?$`)

// IsMSYSTerminal reports whether f is a Cygwin/MSYS pseudo-terminal pipe.
func IsMSYSTerminal(f *os.File) bool {
	handle := windows.Handle(f.Fd())
	if kind, err := windows.GetFileType(handle); err != nil || kind != windows.FILE_TYPE_PIPE {
		return false
	}
	// FILE_NAME_INFO: a byte length followed by the UTF-16 name.
	buf := make([]byte, 4+2*windows.MAX_PATH)
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileNameInfo, &buf[0], uint32(len(buf))); err != nil {
		return false
	}
	size := binary.LittleEndian.Uint32(buf)
	if int(size) > len(buf)-4 {
		return false
	}
	name := windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[4])), size/2))
	return msysPTYPipe.MatchString(name)
}
