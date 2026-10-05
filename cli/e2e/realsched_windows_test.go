//go:build e2e && windows

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

// TestRealTaskScheduler exercises the product's Task Scheduler code against
// the real service, with a stand-in binary instead of "forged daemon" so the
// task never touches the developer's real profile. The stand-in lives under
// a path with spaces and non-ASCII characters on purpose.
func TestRealTaskScheduler(t *testing.T) {
	realPath := strings.Join(filepathListWithout(os.Getenv("PATH"), h.fakeBin), ";")
	// Profiles like C:\Users\José Smith flow into the task name and the
	// staged daemon path.
	probeHome := filepath.Join(h.root, "Forged Probë "+randomHex(3))
	sleeperDir := filepath.Join(h.root, "stand in ñ dir")
	if err := os.MkdirAll(probeHome, 0o700); err != nil {
		t.Fatal(err)
	}
	sleeper := filepath.Join(sleeperDir, "forged.exe")
	if err := goBuild(sleeper, "./e2e/fakes/sleeper", h.buildID); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", realPath)
	t.Setenv("USERPROFILE", probeHome)
	t.Setenv("LOCALAPPDATA", filepath.Join(probeHome, "AppData", "Local"))
	paths := config.DefaultPaths()
	t.Cleanup(func() {
		_ = daemon.UninstallService()
		_ = exec.Command("schtasks", "/Delete", "/TN", "ForgedSSHAgent-"+filepath.Base(probeHome), "/F").Run()
	})

	if err := daemon.InstallService(paths, daemon.RuntimeSpec{Binary: sleeper, Args: []string{"daemon"}}); err != nil {
		t.Fatalf("InstallService: %v", err)
	}
	status, err := daemon.InspectService(paths)
	if err != nil || !status.Installed || !status.ConfigValid || status.Running {
		t.Fatalf("inspect after install: %+v err=%v", status, err)
	}
	stagedRoot := stagedDaemonRoot() + `\`
	if !strings.HasPrefix(status.BinaryPath, stagedRoot) || filepath.Base(status.BinaryPath) != "forged.exe" {
		t.Fatalf("task binary %q, want a staged copy under %q (non-ASCII or spaces mangled?)", status.BinaryPath, stagedRoot)
	}
	// The stand-in writes its report next to the staged copy it runs from.
	sleeperDir = filepath.Dir(status.BinaryPath)

	if err := daemon.StartService(); err != nil {
		t.Fatalf("StartService: %v", err)
	}
	report := filepath.Join(sleeperDir, "sleeper-report.json")
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(report); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task never started the stand-in binary")
		}
		time.Sleep(200 * time.Millisecond)
	}
	data, _ := os.ReadFile(report)
	var launched struct {
		PID          int      `json:"pid"`
		Args         []string `json:"args"`
		VisibleAfter bool     `json:"console_visible_after_detach"`
	}
	if err := json.Unmarshal(data, &launched); err != nil {
		t.Fatal(err)
	}
	t.Logf("task launch report: %s", data)
	if len(launched.Args) != 1 || launched.Args[0] != "daemon" {
		t.Errorf("task args %q, want [daemon]", launched.Args)
	}
	if launched.VisibleAfter {
		t.Error("console window still visible after DetachOwnedConsole")
	}

	status, err = daemon.InspectService(paths)
	if err != nil || !status.Running || !status.PIDKnown || status.PID != launched.PID {
		t.Fatalf("inspect while running: %+v err=%v (launched pid %d)", status, err, launched.PID)
	}
	// EnsureService re-registers the task and re-checks ownership before it
	// restarts, so the running instance must survive re-registration.
	if err := daemon.InstallService(paths, daemon.RuntimeSpec{Binary: sleeper, Args: []string{"daemon"}}); err != nil {
		t.Fatalf("re-registering running task: %v", err)
	}
	status, err = daemon.InspectService(paths)
	if err != nil || !status.Running || !status.PIDKnown || status.PID != launched.PID {
		t.Fatalf("inspect after re-registering a running task: %+v err=%v (launched pid %d)", status, err, launched.PID)
	}

	if err := daemon.StopService(); err != nil {
		t.Fatalf("StopService: %v", err)
	}
	if status, err = daemon.InspectService(paths); err != nil || status.Running {
		t.Fatalf("inspect after stop: %+v err=%v", status, err)
	}
	if err := daemon.UninstallService(); err != nil {
		t.Fatalf("UninstallService: %v", err)
	}
	if installed, err := daemon.ServiceInstalled(); err != nil || installed {
		t.Fatalf("installed after uninstall: %v err=%v", installed, err)
	}
}

func filepathListWithout(list, drop string) []string {
	var out []string
	for _, p := range strings.Split(list, ";") {
		if !strings.EqualFold(filepath.Clean(p), filepath.Clean(drop)) {
			out = append(out, p)
		}
	}
	return out
}
