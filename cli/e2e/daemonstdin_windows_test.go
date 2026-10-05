//go:build e2e && windows

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/itzzritik/forged/cli/internal/platform"
)

// TestDaemonUnderMSYSTerminal runs `forged daemon` the way mintty without
// ConPTY does: stdin is an MSYS pty pipe. It must start instead of blocking
// on that pipe as if it carried a startup password. Runs before the scenario
// installs its service, so the account's pipes are free.
func TestDaemonUnderMSYSTerminal(t *testing.T) {
	name := `\\.\pipe\msys-` + randomHex(8) + `-pty0-from-master-nat`
	ln, err := winio.ListenPipe(name, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if conn, err := ln.Accept(); err == nil {
			t.Cleanup(func() { conn.Close() })
		}
	}()
	var stdin *os.File
	for range 50 {
		if stdin, err = os.OpenFile(name, os.O_RDONLY, 0); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	// A separate profile keeps this daemon's files out of the scenario's home.
	home := filepath.Join(h.root, "msys-home")
	cmd := exec.Command(filepath.Join(h.bin, "forged.exe"), "daemon")
	cmd.Env = append(append([]string(nil), h.env...),
		"USERPROFILE="+home,
		"HOME="+home,
		"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"))
	cmd.Stdin = stdin
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	agent, ctl, err := platform.CurrentUserPipePaths()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		deadline := time.Now().Add(10 * time.Second)
		for platform.IsSocketAlive(ctl) || platform.IsSocketAlive(agent) {
			if time.Now().After(deadline) {
				t.Fatal("daemon pipes still alive after kill")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	deadline := time.Now().Add(20 * time.Second)
	for !platform.IsSocketAlive(ctl) {
		if time.Now().After(deadline) {
			t.Fatal("daemon never served its control pipe; it is blocked reading the MSYS pty as a startup password")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
