//go:build windows

package main

import (
	"os/exec"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Task Scheduler starts the daemon without foreground rights, so prompts it
// triggers open behind the active app.

const helloPromptClass = "Credential Dialog Xaml Host"

var (
	procFindWindowExW       = user32.NewProc("FindWindowExW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procPeekMessageW        = user32.NewProc("PeekMessageW")
)

// runPrompt runs a prompt script and brings its window to the front once it
// appears.
func runPrompt(cmd *exec.Cmd, find func(pid uint32) uintptr) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := uint32(cmd.Process.Pid)
	stop := make(chan struct{})
	raised := make(chan struct{})
	go func() {
		defer close(raised)
		raiseWhenShown(stop, func() uintptr { return find(pid) })
	}()
	err := cmd.Wait()
	close(stop)
	<-raised
	return err
}

// raiseWhenShown waits for the prompt window and raises it until it is in
// front, then stops so the user can still switch away.
func raiseWhenShown(stop <-chan struct{}, find func() uintptr) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var hwnd uintptr
	for attempts := 0; attempts < 10; {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if hwnd == 0 {
			if hwnd = find(); hwnd == 0 {
				continue
			}
		}
		if foreground, _, _ := procGetForegroundWindow.Call(); foreground == hwnd {
			return
		}
		bringToFront(hwnd)
		attempts++
	}
}

// bringToFront shares the foreground thread's input state for the call, which
// lets SetForegroundWindow past the foreground lock. Injecting a key (the
// usual ALT trick) is unsafe here: once the elevated Windows Hello dialog is in
// front, UIPI drops the key-up and the key stays held system-wide.
func bringToFront(hwnd uintptr) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	foreground, _, _ := procGetForegroundWindow.Call()
	if foreground == hwnd {
		return
	}
	// AttachThreadInput needs this thread to have a message queue.
	var msg winMsg
	_, _, _ = procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
	current := windows.GetCurrentThreadId()
	target, _ := windows.GetWindowThreadProcessId(windows.HWND(foreground), nil)
	if target != 0 && target != current {
		if attached, _, _ := procAttachThreadInput.Call(uintptr(current), uintptr(target), 1); attached != 0 {
			defer procAttachThreadInput.Call(uintptr(current), uintptr(target), 0)
		}
	}
	_, _, _ = procSetForegroundWindow.Call(hwnd)
}

// helloPromptFinder matches a Windows Hello dialog that was not already open,
// so another app's prompt is never raised.
func helloPromptFinder() func(uint32) uintptr {
	existing := findWindow(helloPromptClass, "", 0, 0)
	return func(uint32) uintptr { return findWindow(helloPromptClass, "", 0, existing) }
}

func passwordPromptFinder(pid uint32) uintptr {
	return findWindow("", "Forged", pid, 0)
}

// findWindow returns a visible top-level window matching class and title
// (empty matches any), owned by pid when non-zero, other than exclude.
func findWindow(class, title string, pid uint32, exclude uintptr) uintptr {
	var after uintptr
	for {
		hwnd, _, _ := procFindWindowExW.Call(0, after, utf16Arg(class), utf16Arg(title))
		if hwnd == 0 {
			return 0
		}
		after = hwnd
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 || hwnd == exclude {
			continue
		}
		if pid != 0 {
			var owner uint32
			if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &owner); err != nil || owner != pid {
				continue
			}
		}
		return hwnd
	}
}

func utf16Arg(s string) uintptr {
	if s == "" {
		return 0
	}
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return 0
	}
	return uintptr(unsafe.Pointer(p))
}
