//go:build e2e && windows

// sleeper stands in for "forged daemon" under the real Task Scheduler and
// reports whether the daemon's console detach left a window behind.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/sys/windows"
)

func main() {
	platform.DetachOwnedConsole()
	hwnd, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	report := map[string]any{
		"pid":                          os.Getpid(),
		"args":                         os.Args[1:],
		"console_visible_after_detach": hwnd != 0 && windows.IsWindowVisible(windows.HWND(hwnd)),
	}
	exe, _ := os.Executable()
	data, _ := json.Marshal(report)
	_ = os.WriteFile(filepath.Join(filepath.Dir(exe), "sleeper-report.json"), data, 0o600)
	time.Sleep(2 * time.Minute)
}
