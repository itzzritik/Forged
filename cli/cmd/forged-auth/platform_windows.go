//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/sys/windows"
)

func providerName() string { return "windows-hello" }

func authorize(ctx context.Context, action sensitiveauth.Action) string {
	return runWindowsHello(ctx, windowsHelloAuthorizeScript, action.NativeReason(), "failed",
		map[int]string{2: "unavailable_by_environment", 3: "canceled"})
}

func status(ctx context.Context) string {
	return runWindowsHello(ctx, windowsHelloStatusScript, "", "broken",
		map[int]string{2: "unavailable_by_environment", 4: "unavailable_by_platform"})
}

// Absolute path: only Windows PowerShell 5.1 has the WinRT projection, and a
// PATH-resolved powershell.exe could answer "verified".
func runWindowsHello(ctx context.Context, script, reason, fallback string, exitCodes map[int]string) string {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return "unavailable_by_environment"
	}
	shell := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, shell,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", platform.PowerShellEncodedCommand(script))
	cmd.Env = append(os.Environ(), "FORGED_AUTH_REASON="+reason)
	platform.HideChildConsole(cmd)
	err = cmd.Run()
	switch {
	case ctx.Err() != nil:
		return "canceled"
	case err == nil:
		return "ok"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if result, ok := exitCodes[exitErr.ExitCode()]; ok {
			return result
		}
	} else if errors.Is(err, os.ErrNotExist) {
		return "unavailable_by_environment"
	}
	return fallback
}

// Add-Type against System32\WinMetadata\Windows.winmd fails on current Windows.
const windowsHelloPrelude = `
$ErrorActionPreference = 'Stop'
try {
  Add-Type -AssemblyName System.Runtime.WindowsRuntime
  $null = [Windows.Security.Credentials.UI.UserConsentVerifier, Windows.Security.Credentials.UI, ContentType = WindowsRuntime]
  $asTask = [System.WindowsRuntimeSystemExtensions].GetMethods() |
    Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' } |
    Select-Object -First 1
  if ($null -eq $asTask) { exit 2 }
} catch {
  exit 2
}
function Wait-WinRT($operation, [Type]$resultType) {
  $task = $asTask.MakeGenericMethod($resultType).Invoke($null, @($operation))
  $task.Wait(-1) | Out-Null
  return $task.Result
}
$verifier = [Windows.Security.Credentials.UI.UserConsentVerifier]
`

const windowsHelloStatusScript = windowsHelloPrelude + `
try {
  $availability = Wait-WinRT ($verifier::CheckAvailabilityAsync()) ([Windows.Security.Credentials.UI.UserConsentVerifierAvailability])
} catch {
  exit 1
}
switch ($availability.ToString()) {
  'Available' { exit 0 }
  'DeviceBusy' { exit 1 }
  'RetriesExhausted' { exit 1 }
  'Canceled' { exit 1 }
  default { exit 4 }
}
`

const windowsHelloAuthorizeScript = windowsHelloPrelude + `
try {
  $availability = Wait-WinRT ($verifier::CheckAvailabilityAsync()) ([Windows.Security.Credentials.UI.UserConsentVerifierAvailability])
  if ($availability.ToString() -ne 'Available') { exit 2 }
  $result = Wait-WinRT ($verifier::RequestVerificationAsync([string]$env:FORGED_AUTH_REASON)) ([Windows.Security.Credentials.UI.UserConsentVerificationResult])
} catch {
  exit 1
}
switch ($result.ToString()) {
  'Verified' { exit 0 }
  'Canceled' { exit 3 }
  'DeviceNotPresent' { exit 2 }
  'NotConfiguredForUser' { exit 2 }
  'DisabledByPolicy' { exit 2 }
  default { exit 1 }
}
`

func startLockLoop(ctx context.Context, onLock func()) {
	if onLock == nil {
		return
	}
	lockMonitorOnLock = onLock
	// Backoff: WTS registration fails until Remote Desktop Services starts.
	delay := time.Second
	for ctx.Err() == nil {
		if runLockMonitor(ctx) {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, 30*time.Second)
	}
}

const (
	wmClose            = 0x0010
	wmPowerBroadcast   = 0x0218
	wmWTSSessionChange = 0x02B1

	pbtAPMSuspend = 0x0004

	wtsConsoleDisconnect = 0x2
	wtsRemoteDisconnect  = 0x4
	wtsSessionLock       = 0x7
)

var (
	user32                           = windows.NewLazySystemDLL("user32.dll")
	wtsapi32                         = windows.NewLazySystemDLL("wtsapi32.dll")
	procRegisterClassExW             = user32.NewProc("RegisterClassExW")
	procCreateWindowExW              = user32.NewProc("CreateWindowExW")
	procDestroyWindow                = user32.NewProc("DestroyWindow")
	procDefWindowProcW               = user32.NewProc("DefWindowProcW")
	procGetMessageW                  = user32.NewProc("GetMessageW")
	procDispatchMessageW             = user32.NewProc("DispatchMessageW")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procPostQuitMessage              = user32.NewProc("PostQuitMessage")
	procRegisterSuspendResumeNotif   = user32.NewProc("RegisterSuspendResumeNotification")
	procUnregisterSuspendResumeNotif = user32.NewProc("UnregisterSuspendResumeNotification")
	procWTSRegisterSessionNotif      = wtsapi32.NewProc("WTSRegisterSessionNotification")
	procWTSUnRegisterSessionNotif    = wtsapi32.NewProc("WTSUnRegisterSessionNotification")
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// Only the single startLockLoop goroutine touches these.
var (
	lockMonitorOnLock     func()
	lockMonitorProc       = windows.NewCallback(handleLockMonitorMessage)
	lockMonitorClass, _   = windows.UTF16PtrFromString("ForgedAuthLockMonitor")
	lockMonitorRegistered bool
)

func handleLockMonitorMessage(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case wmWTSSessionChange:
		switch wParam {
		case wtsSessionLock, wtsConsoleDisconnect, wtsRemoteDisconnect:
			lockMonitorOnLock()
		}
		return 0
	case wmPowerBroadcast:
		if wParam == pbtAPMSuspend {
			lockMonitorOnLock()
		}
		return 1
	case wmClose:
		_, _, _ = procDestroyWindow.Call(hwnd)
		_, _, _ = procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

// Session notifications go to the registered window directly; broadcast
// power messages never reach a message-only window, hence the suspend hook.
func runLockMonitor(ctx context.Context) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if !lockMonitorRegistered {
		class := wndClassEx{wndProc: lockMonitorProc, className: lockMonitorClass}
		class.size = uint32(unsafe.Sizeof(class))
		atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if lockMonitorRegistered = atom != 0; !lockMonitorRegistered {
			return false
		}
	}
	const hwndMessage = ^uintptr(2) // HWND_MESSAGE (-3)
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(lockMonitorClass)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, 0, 0)
	if hwnd == 0 {
		return false
	}
	if ok, _, _ := procWTSRegisterSessionNotif.Call(hwnd, 0); ok == 0 {
		_, _, _ = procDestroyWindow.Call(hwnd)
		return false
	}
	defer procWTSUnRegisterSessionNotif.Call(hwnd)
	if handle, _, _ := procRegisterSuspendResumeNotif.Call(hwnd, 0); handle != 0 {
		defer procUnregisterSuspendResumeNotif.Call(handle)
	}

	stop := context.AfterFunc(ctx, func() {
		_, _, _ = procPostMessageW.Call(hwnd, wmClose, 0, 0)
	})
	defer stop()

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return true
		}
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
