//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// deleteAfterExit removes what Windows would not let a running daemon delete
// (its own exe, open logs) once process pid has exited.
func deleteAfterExit(pid int, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	quoted := make([]string, len(paths))
	for i, path := range paths {
		quoted[i] = "'" + strings.ReplaceAll(path, "'", "''") + "'"
	}
	script := fmt.Sprintf("Wait-Process -Id %d -Timeout 120 -ErrorAction SilentlyContinue; Remove-Item -LiteralPath %s -Recurse -Force -ErrorAction SilentlyContinue", pid, strings.Join(quoted, ","))
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting cleanup helper: %w", err)
	}
	return cmd.Process.Release()
}
