//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/sys/windows"
)

func providerName() string { return "windows-hello" }

func authorize(ctx context.Context, action sensitiveauth.Action) string {
	cmd, err := powerShell(ctx, windowsHelloAuthorizeScript, action.NativeReason())
	if err != nil {
		return "unavailable_by_environment"
	}
	return scriptStatus(ctx, runPrompt(cmd, helloPromptFinder()), "failed",
		map[int]string{2: "unavailable_by_environment", 3: "canceled"})
}

func status(ctx context.Context) string {
	cmd, err := powerShell(ctx, windowsHelloStatusScript, "")
	if err != nil {
		return "unavailable_by_environment"
	}
	return scriptStatus(ctx, cmd.Run(), "broken",
		map[int]string{2: "unavailable_by_environment", 4: "unavailable_by_platform"})
}

// collectPassword shows the master-password dialog and returns the password
// base64-encoded; the script emits base64 because PowerShell writes stdout in
// the OEM code page, which would mangle non-ASCII passwords.
func collectPassword(ctx context.Context, reason string) (string, string) {
	cmd, err := powerShell(ctx, passwordPromptScript, reason)
	if err != nil {
		return "unavailable_by_environment", ""
	}
	// HideWindow puts SW_HIDE in STARTUPINFO, which Windows applies to the
	// child's first ShowWindow: the dialog itself.
	cmd.SysProcAttr.HideWindow = false
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = runPrompt(cmd, passwordPromptFinder)
	out := stdout.Bytes()
	defer clear(out)
	if status := scriptStatus(ctx, err, "failed", map[int]string{2: "unavailable_by_environment", 3: "canceled"}); status != "ok" {
		return status, ""
	}
	secret := strings.TrimSpace(string(out))
	password, err := base64.StdEncoding.DecodeString(secret)
	clear(password)
	if err != nil {
		return "failed", ""
	}
	return "ok", secret
}

// Absolute path: only Windows PowerShell 5.1 has the WinRT projection, and a
// PATH-resolved powershell.exe could answer "verified".
func powerShell(ctx context.Context, script, reason string) (*exec.Cmd, error) {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	shell := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, shell,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", platform.PowerShellEncodedCommand(script))
	cmd.Env = append(os.Environ(), "FORGED_AUTH_REASON="+reason)
	platform.HideChildConsole(cmd)
	return cmd, nil
}

func scriptStatus(ctx context.Context, err error, fallback string, exitCodes map[int]string) string {
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

// Exit 2: WinForms is unavailable (e.g. Constrained Language Mode or no
// desktop). Exit 3: the user canceled. TopMost because a background helper
// cannot always take the foreground.
const passwordPromptScript = `
$ErrorActionPreference = 'Stop'
try {
  Add-Type -AssemblyName System.Windows.Forms, System.Drawing
  [System.Windows.Forms.Application]::EnableVisualStyles()
  $font = [System.Drawing.SystemFonts]::MessageBoxFont
  $form = New-Object System.Windows.Forms.Form
} catch {
  exit 2
}
$form.SuspendLayout()
$form.AutoScaleDimensions = New-Object System.Drawing.SizeF(96, 96)
$form.AutoScaleMode = [System.Windows.Forms.AutoScaleMode]::Dpi
$form.Text = 'Forged'
$form.Font = $font
$form.FormBorderStyle = [System.Windows.Forms.FormBorderStyle]::FixedDialog
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.StartPosition = [System.Windows.Forms.FormStartPosition]::CenterScreen
$form.TopMost = $true
$form.AutoSize = $true
$form.AutoSizeMode = [System.Windows.Forms.AutoSizeMode]::GrowAndShrink

$layout = New-Object System.Windows.Forms.TableLayoutPanel
$layout.AutoSize = $true
$layout.ColumnCount = 1
$layout.Padding = New-Object System.Windows.Forms.Padding(16)

$title = New-Object System.Windows.Forms.Label
$title.Text = 'Forged is locked'
$title.AutoSize = $true
$title.Font = New-Object System.Drawing.Font($font.FontFamily, ($font.Size + 3), [System.Drawing.FontStyle]::Bold)
$title.Margin = New-Object System.Windows.Forms.Padding(0, 0, 0, 6)

$message = New-Object System.Windows.Forms.Label
$message.Text = if ($env:FORGED_AUTH_REASON) { $env:FORGED_AUTH_REASON } else { 'Enter your Forged master password to continue.' }
$message.AutoSize = $true
$message.MaximumSize = New-Object System.Drawing.Size(320, 0)
$message.Margin = New-Object System.Windows.Forms.Padding(0, 0, 0, 12)

$box = New-Object System.Windows.Forms.TextBox
$box.UseSystemPasswordChar = $true
$box.Width = 320
$box.Margin = New-Object System.Windows.Forms.Padding(0, 0, 0, 16)

$unlock = New-Object System.Windows.Forms.Button
$unlock.Text = 'Unlock'
$unlock.DialogResult = [System.Windows.Forms.DialogResult]::OK
$cancel = New-Object System.Windows.Forms.Button
$cancel.Text = 'Cancel'
$cancel.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$buttons = New-Object System.Windows.Forms.FlowLayoutPanel
$buttons.FlowDirection = [System.Windows.Forms.FlowDirection]::RightToLeft
$buttons.AutoSize = $true
$buttons.Dock = [System.Windows.Forms.DockStyle]::Fill
$buttons.Margin = New-Object System.Windows.Forms.Padding(0)
$buttons.Controls.AddRange(@($cancel, $unlock))

$layout.Controls.AddRange(@($title, $message, $box, $buttons))
$form.Controls.Add($layout)
$form.AcceptButton = $unlock
$form.CancelButton = $cancel
$form.Add_Shown({ $form.Activate(); $box.Focus() })
$form.ResumeLayout($true)

if ($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 3 }
[Console]::Out.Write([Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($box.Text)))
exit 0
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
