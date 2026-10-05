//go:build e2e && windows

package e2e

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW = user32.NewProc("FindWindowExW")
	procPostMessageW  = user32.NewProc("PostMessageW")
)

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
