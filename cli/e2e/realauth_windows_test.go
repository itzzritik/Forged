//go:build e2e && windows

package e2e

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/sys/windows"
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW     = user32.NewProc("FindWindowExW")
	procPostMessageW      = user32.NewProc("PostMessageW")
	procSendMessageW      = user32.NewProc("SendMessageW")
	procEnumChildWindows  = user32.NewProc("EnumChildWindows")
	procGetClassNameW     = user32.NewProc("GetClassNameW")
	procGetWindowTextW    = user32.NewProc("GetWindowTextW")
	procIsWindowVisible   = user32.NewProc("IsWindowVisible")
	procIsWindow          = user32.NewProc("IsWindow")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
)

// TestRealPasswordPrompt drives the shipped forged-auth master-password
// dialog. It shows a window on the desktop, so it only runs with
// FORGED_E2E_UI=1.
func TestRealPasswordPrompt(t *testing.T) {
	if os.Getenv("FORGED_E2E_UI") != "1" {
		t.Skip("set FORGED_E2E_UI=1 to drive the real password dialog")
	}
	cmd := exec.Command(filepath.Join(h.realAuth, "forged-auth.exe"))
	cmd.Env = h.env
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		_ = cmd.Wait()
	})
	responses := readHelper(stdout)
	send := func(req sensitiveauth.HelperRequest) {
		b, _ := json.Marshal(req)
		_, _ = stdin.Write(append(b, '\n'))
	}
	isResponse := func(id string) func(sensitiveauth.HelperResponse) bool {
		return func(r sensitiveauth.HelperResponse) bool { return r.ID == id }
	}

	// Unlock: a non-ASCII password survives PowerShell's OEM-code-page stdout.
	send(sensitiveauth.NewCollectPasswordRequest("pw-unlock", "Enter your Forged master password to keep SSH working."))
	dialog := findPasswordDialog(t)
	if exStyle, _, _ := procGetWindowLongPtrW.Call(dialog, uintptr(0xFFFFFFEC)); exStyle&0x8 == 0 { // GWL_EXSTYLE, WS_EX_TOPMOST
		t.Error("password dialog is not topmost")
	}
	const password = "pässwörd ✓ 123"
	text, _ := windows.UTF16PtrFromString(password)
	_, _, _ = procSendMessageW.Call(dialogChild(t, dialog, "EDIT", ""), 0x000C, 0, uintptr(unsafe.Pointer(text))) // WM_SETTEXT
	_, _, _ = procSendMessageW.Call(dialogChild(t, dialog, "BUTTON", "Unlock"), 0x00F5, 0, 0)                     // BM_CLICK
	resp := waitHelper(t, responses, 10*time.Second, isResponse("pw-unlock"))
	got, err := base64.StdEncoding.DecodeString(resp.Secret)
	if resp.Status != "ok" || err != nil || string(got) != password {
		t.Fatalf("unlock: status %q, secret %q (%v)", resp.Status, got, err)
	}

	// Cancel button.
	send(sensitiveauth.NewCollectPasswordRequest("pw-button", ""))
	_, _, _ = procSendMessageW.Call(dialogChild(t, findPasswordDialog(t), "BUTTON", "Cancel"), 0x00F5, 0, 0)
	if resp := waitHelper(t, responses, 10*time.Second, isResponse("pw-button")); resp.Status != "canceled" {
		t.Fatalf("cancel button: status %q", resp.Status)
	}

	// A broker cancel (e.g. a screen lock) closes the dialog.
	send(sensitiveauth.NewCollectPasswordRequest("pw-ipc", ""))
	dialog = findPasswordDialog(t)
	send(sensitiveauth.HelperRequest{ID: "pw-ipc", Type: "cancel"})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if open, _, _ := procIsWindow.Call(dialog); open == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("password dialog still open after cancel")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func windowText(hwnd uintptr, get *windows.LazyProc) string {
	buf := make([]uint16, 256)
	_, _, _ = get.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func findPasswordDialog(t *testing.T) uintptr {
	t.Helper()
	title, _ := windows.UTF16PtrFromString("Forged")
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var after uintptr
		for {
			hwnd, _, _ := procFindWindowExW.Call(0, after, 0, uintptr(unsafe.Pointer(title)))
			if hwnd == 0 {
				break
			}
			visible, _, _ := procIsWindowVisible.Call(hwnd)
			if visible != 0 && strings.HasPrefix(windowText(hwnd, procGetClassNameW), "WindowsForms10.Window") {
				return hwnd
			}
			after = hwnd
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("visible password dialog not found")
	return 0
}

// dialogChild finds a WinForms control by class kind (EDIT, BUTTON) and,
// when given, its text.
func dialogChild(t *testing.T, dialog uintptr, kind, text string) uintptr {
	t.Helper()
	var found uintptr
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if strings.Contains(windowText(hwnd, procGetClassNameW), "."+kind+".") && (text == "" || windowText(hwnd, procGetWindowTextW) == text) {
			found = hwnd
			return 0
		}
		return 1
	})
	_, _, _ = procEnumChildWindows.Call(dialog, callback, 0)
	if found == 0 {
		t.Fatalf("password dialog has no %s %q", kind, text)
	}
	return found
}

// TestRealAuthHelper checks the shipped forged-auth without prompting: its
// Windows Hello status probe, and that session-lock and suspend messages
// reaching its monitor window become session_locked events.
func TestRealAuthHelper(t *testing.T) {
	cmd := exec.Command(filepath.Join(h.realAuth, "forged-auth.exe"))
	cmd.Env = h.env
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Error("forged-auth did not exit after stdin closed")
		}
	})
	responses := readHelper(stdout)

	send := func(req sensitiveauth.HelperRequest) {
		b, _ := json.Marshal(req)
		_, _ = stdin.Write(append(b, '\n'))
	}
	send(sensitiveauth.HelperRequest{ID: "status-1", Type: "status"})
	resp := waitHelper(t, responses, 30*time.Second, func(r sensitiveauth.HelperResponse) bool { return r.ID == "status-1" })
	t.Logf("windows hello status: %s", resp.Status)
	switch resp.Status {
	case "ok", "unavailable_by_platform":
	default:
		t.Errorf("status %q: the Windows Hello probe itself failed (expected ok or unavailable_by_platform)", resp.Status)
	}

	hwnd := findMonitorWindow(t, uint32(cmd.Process.Pid))
	const wmWTSSessionChange, wtsSessionLock = 0x02B1, 0x7
	const wmPowerBroadcast, pbtAPMSuspend = 0x0218, 0x4
	for _, m := range []struct {
		name        string
		msg, wParam uintptr
	}{
		{"session lock", wmWTSSessionChange, wtsSessionLock},
		{"suspend", wmPowerBroadcast, pbtAPMSuspend},
	} {
		_, _, _ = procPostMessageW.Call(hwnd, m.msg, m.wParam, 0)
		waitHelper(t, responses, 5*time.Second, func(r sensitiveauth.HelperResponse) bool {
			return r.Type == "event" && r.Status == "session_locked"
		})
		t.Logf("%s -> session_locked", m.name)
	}
}

func readHelper(r io.Reader) <-chan sensitiveauth.HelperResponse {
	ch := make(chan sensitiveauth.HelperResponse, 16)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			var resp sensitiveauth.HelperResponse
			if json.Unmarshal(scanner.Bytes(), &resp) == nil {
				ch <- resp
			}
		}
	}()
	return ch
}

func waitHelper(t *testing.T, ch <-chan sensitiveauth.HelperResponse, timeout time.Duration, match func(sensitiveauth.HelperResponse) bool) sensitiveauth.HelperResponse {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatal("helper exited")
			}
			if match(resp) {
				return resp
			}
		case <-deadline:
			t.Fatal("timed out waiting for helper response")
		}
	}
}

func findMonitorWindow(t *testing.T, pid uint32) uintptr {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("ForgedAuthLockMonitor")
	const hwndMessage = ^uintptr(2)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var after uintptr
		for {
			hwnd, _, _ := procFindWindowExW.Call(hwndMessage, after, uintptr(unsafe.Pointer(class)), 0)
			if hwnd == 0 {
				break
			}
			var owner uint32
			_, _ = windows.GetWindowThreadProcessId(windows.HWND(hwnd), &owner)
			if owner == pid {
				return hwnd
			}
			after = hwnd
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("forged-auth lock monitor window not found")
	return 0
}
