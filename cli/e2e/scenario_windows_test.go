//go:build e2e && windows

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/platform"
)

const e2ePassword = "correct horse battery staple"

type scenario struct {
	paths     config.Paths
	taskName  string
	keyName   string
	publicKey string
	server    *sshServer
}

// TestWindowsScenario walks one isolated Windows user from first launch to
// upgrade. Steps share state and stop at the first failure.
func TestWindowsScenario(t *testing.T) {
	s := &scenario{
		paths:    config.DefaultPaths(),
		taskName: "ForgedSSHAgent-" + filepath.Base(h.home),
		keyName:  "e2e-key",
	}
	s.server = startSSHServer(t)
	steps := []struct {
		name string
		fn   func(*testing.T, *scenario)
	}{
		{"first run creates vault and a healthy service", stepFirstRun},
		{"generate key in the TUI", stepGenerateKey},
		{"ssh.exe authenticates through the agent pipe", stepSSHLogin},
		{"git over SSH uses Win32-OpenSSH and the agent", stepGitOverSSH},
		{"git commit signing through forged-sign", stepCommitSigning},
		{"locked vault unlocks through System Auth for ssh", stepLockSystemAuth},
		{"canceled System Auth denies ssh", stepSystemAuthCanceled},
		{"OS session lock clears the session", stepOSLockEvent},
		{"daemon restart at logon keeps working", stepDaemonRestart},
		{"upgraded binary refreshes the running daemon", stepUpgrade},
		{"doctor --fix repairs a broken install", stepDoctorFix},
		{"disable and re-enable SSH integration", stepToggleSSH},
		{"forged logs follows the live daemon log", stepLogs},
	}
	for _, step := range steps {
		if !t.Run(step.name, func(t *testing.T) { step.fn(t, s) }) {
			t.Fatalf("stopping after failed step %q", step.name)
		}
	}
}

func stepFirstRun(t *testing.T, s *scenario) {
	createVault(t, h.forged(t))

	xmlFile := filepath.Join(h.schedDir, s.taskName+".xml")
	data, err := os.ReadFile(xmlFile)
	if err != nil || !bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		t.Errorf("task XML must be UTF-16LE with BOM for schtasks (err=%v)", err)
	}
	managed := readFile(t, s.paths.SSHManagedConfig())
	if !strings.Contains(managed, "IdentityAgent //./pipe/forged-agent-v1-") {
		t.Errorf("managed SSH config lacks forward-slash pipe IdentityAgent:\n%s", managed)
	}
	if user := readFile(t, s.paths.SSHUserConfig()); !strings.Contains(user, "forged.conf") {
		t.Errorf("~/.ssh/config lacks the Forged Include:\n%s", user)
	}
	if !fileExists(filepath.Join(s.paths.AuthDir(), "local-unlock-a.dpapi")) && !fileExists(filepath.Join(s.paths.AuthDir(), "local-unlock-b.dpapi")) {
		t.Error("no DPAPI local-unlock enrollment after password setup")
	}
	assertServiceOwned(t, s)
	if status, err := actions.LoadRuntimeStatus(s.paths); err != nil || !status.Unlocked {
		t.Errorf("runtime status after setup: %+v err=%v", status, err)
	}
}

func stepGenerateKey(t *testing.T, s *scenario) {
	p := h.forged(t)
	mustSee(t, p, 20*time.Second, "DASHBOARD", "Generate")
	_ = p.Send("\x1b[B")
	time.Sleep(300 * time.Millisecond)
	_ = p.Send("\r")
	mustSee(t, p, 10*time.Second, "Create a new SSH key")
	_ = p.Type(s.keyName)
	_ = p.Send("\r")
	mustSee(t, p, 20*time.Second, s.keyName, "SHA256:")
	quitTUI(t, p)

	pub, err := actions.ExportPublicKey(s.paths, s.keyName)
	if err != nil {
		t.Fatalf("export public key: %v", err)
	}
	s.publicKey = pub.PublicKey
	s.server.Authorize(t, s.publicKey)
}

func stepSSHLogin(t *testing.T, s *scenario) {
	out, err := s.ssh(t)
	if err != nil || !strings.Contains(out, "forged-e2e-ok user=e2e") {
		t.Fatalf("ssh login failed: %v\n%s", err, out)
	}
	out, err = h.runEnv(t, []string{"SSH_AUTH_SOCK=" + s.paths.AgentSocket()}, h.home, "ssh-add", "-L")
	if err != nil || strings.TrimSpace(out) != strings.TrimSpace(s.publicKey)+" "+s.keyName {
		t.Errorf("ssh-add -L via IdentityAgent pipe: %v\n%s", err, out)
	}
}

func stepGitOverSSH(t *testing.T, s *scenario) {
	// Git for Windows would otherwise run its bundled MSYS ssh, which ignores
	// the Include and cannot open the agent pipe.
	command, _ := h.run(t, h.home, "git", "config", "--global", "core.sshCommand")
	if !strings.EqualFold(filepath.FromSlash(strings.TrimSpace(command)), filepath.Join(h.fakeBin, "ssh.exe")) {
		t.Fatalf("core.sshCommand = %q, want the Win32-OpenSSH client on PATH", strings.TrimSpace(command))
	}
	out, err := h.runEnv(t, []string{"GIT_TERMINAL_PROMPT=0"}, h.home, "git", "ls-remote", "ssh://e2e@127.0.0.1:"+s.server.Port()+"/repo.git")
	if err != nil {
		t.Fatalf("git ls-remote over ssh: %v\n%s", err, out)
	}
	for _, cmd := range s.server.Commands() {
		if strings.HasPrefix(cmd, "git-upload-pack") {
			return
		}
	}
	t.Fatalf("server never saw git-upload-pack; commands: %q", s.server.Commands())
}

func stepCommitSigning(t *testing.T, s *scenario) {
	status, err := actions.EnableCommitSigning(s.paths, s.keyName)
	if err != nil {
		t.Fatalf("enable commit signing: %v", err)
	}
	t.Logf("signing status: %+v", status)
	program, _ := h.run(t, h.home, "git", "config", "--global", "gpg.ssh.program")
	if !strings.EqualFold(filepath.Clean(strings.TrimSpace(program)), filepath.Join(h.bin, "forged-sign.exe")) {
		t.Errorf("gpg.ssh.program = %q", strings.TrimSpace(program))
	}

	repo := filepath.Join(h.root, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Forged E2E"},
		{"config", "user.email", "e2e@forged.invalid"},
		{"commit", "--allow-empty", "-q", "-m", "signed by forged"},
	} {
		if out, err := h.run(t, repo, "git", args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := h.run(t, repo, "git", "verify-commit", "-v", "HEAD")
	if err != nil || !strings.Contains(out, "Good \"git\" signature") {
		t.Fatalf("git verify-commit: %v\n%s", err, out)
	}
	loaded, err := actions.LoadCommitSigningStatus(s.paths)
	if err != nil {
		t.Errorf("load signing status: %v", err)
	}
	t.Logf("loaded signing status: %+v", loaded)
}

func stepLockSystemAuth(t *testing.T, s *scenario) {
	if err := actions.LockSensitive(s.paths); err != nil {
		t.Fatalf("lock: %v", err)
	}
	waitUnlocked(t, s, false)
	h.setAuth(t, "result", "ok")
	out, err := s.ssh(t)
	if err != nil || !strings.Contains(out, "forged-e2e-ok") {
		t.Fatalf("ssh after lock with System Auth ok: %v\n%s", err, out)
	}
	if !strings.Contains(readFileQuiet(filepath.Join(h.authDir, "requests.log")), "authorize") {
		t.Error("System Auth helper was never asked to authorize")
	}
	waitUnlocked(t, s, true)
}

func stepSystemAuthCanceled(t *testing.T, s *scenario) {
	if err := actions.LockSensitive(s.paths); err != nil {
		t.Fatalf("lock: %v", err)
	}
	waitUnlocked(t, s, false)
	h.setAuth(t, "result", "canceled")
	defer h.setAuth(t, "result", "ok")
	if out, err := s.ssh(t); err == nil {
		t.Fatalf("ssh succeeded although System Auth was canceled:\n%s", out)
	}
}

func stepOSLockEvent(t *testing.T, s *scenario) {
	// Retrying every second must not keep re-arming the 10s cancel cooldown.
	h.setAuth(t, "result", "ok")
	if out, err := s.sshEventually(t, 20*time.Second); err != nil {
		t.Fatalf("ssh to re-establish session: %v\n%s", err, out)
	}
	waitUnlocked(t, s, true)
	h.setAuth(t, "lock", "1")
	waitUnlocked(t, s, false)
}

func stepDaemonRestart(t *testing.T, s *scenario) {
	before := daemonPID(t, s)
	if out, err := h.run(t, h.home, "schtasks", "/End", "/TN", s.taskName); err != nil {
		t.Fatalf("end task: %v\n%s", err, out)
	}
	waitPipes(t, s, false)
	if out, err := h.run(t, h.home, "schtasks", "/Run", "/TN", s.taskName); err != nil {
		t.Fatalf("run task: %v\n%s", err, out)
	}
	waitPipes(t, s, true)
	if after := daemonPID(t, s); after == before {
		t.Errorf("daemon PID unchanged after restart (%d)", after)
	}
	h.setAuth(t, "result", "ok")
	if out, err := s.sshEventually(t, 20*time.Second); err != nil {
		t.Fatalf("ssh after daemon restart: %v\n%s", err, out)
	}
	assertServiceOwned(t, s)
}

func stepUpgrade(t *testing.T, s *scenario) {
	// The daemon runs a staged copy, so an npm/zip upgrade can overwrite the
	// installed binaries in place while it is running.
	before, err := daemon.InspectService(s.paths)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(before.BinaryPath, stagedDaemonRoot()+`\`) {
		t.Fatalf("service runs %q, not a staged copy", before.BinaryPath)
	}
	exe := filepath.Join(h.bin, "forged.exe")
	if image := processImage(uint32(daemonPID(t, s))); strings.EqualFold(image, exe) {
		t.Fatalf("daemon runs the installed binary %s directly", image)
	}
	newBuild := h.buildID + "-next"
	if err := goBuild(exe, "./cmd/forged", newBuild); err != nil {
		t.Fatal(err)
	}
	out, err := h.run(t, h.home, exe, "__daemon-freshen")
	if err != nil || !strings.Contains(out, "refreshed") {
		t.Fatalf("__daemon-freshen: %v\n%s", err, out)
	}
	if got, err := daemon.RunningBuildID(s.paths); err != nil || got != newBuild {
		t.Fatalf("running build %q err=%v, want %q", got, err, newBuild)
	}
	assertServiceOwned(t, s)
	after, _ := daemon.InspectService(s.paths)
	if after.BinaryPath == before.BinaryPath {
		t.Errorf("upgrade kept the old staged binary %s", after.BinaryPath)
	}
	if out, err := s.sshEventually(t, 20*time.Second); err != nil {
		t.Fatalf("ssh after upgrade: %v\n%s", err, out)
	}
}

func stepDoctorFix(t *testing.T, s *scenario) {
	if out, err := h.run(t, h.home, "schtasks", "/End", "/TN", s.taskName); err != nil {
		t.Fatalf("end task: %v\n%s", err, out)
	}
	if err := os.WriteFile(s.paths.SSHUserConfig(), []byte("Host example\n    User nobody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitPipes(t, s, false)

	p := h.forged(t, "doctor", "--fix")
	mustSee(t, p, 40*time.Second, "System healthy")
	quitTUI(t, p)
	waitPipes(t, s, true)
	user := readFile(t, s.paths.SSHUserConfig())
	if !strings.Contains(user, "forged.conf") || !strings.Contains(user, "Host example") {
		t.Errorf("doctor --fix did not restore the Include while keeping user config:\n%s", user)
	}
	if out, err := s.sshEventually(t, 20*time.Second); err != nil {
		t.Fatalf("ssh after doctor --fix: %v\n%s", err, out)
	}
}

func stepToggleSSH(t *testing.T, s *scenario) {
	if err := actions.DisableSSHAgent(s.paths); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if config.IsSSHAgentEnabled(s.paths) {
		t.Error("SSH integration still enabled after disable")
	}
	if out, err := s.ssh(t); err == nil {
		t.Errorf("ssh still authenticated through Forged after disabling integration:\n%s", out)
	}
	if err := actions.EnableSSHAgent(s.paths); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if out, err := s.sshEventually(t, 20*time.Second); err != nil {
		t.Fatalf("ssh after re-enable: %v\n%s", err, out)
	}
}

func stepLogs(t *testing.T, s *scenario) {
	p := h.forged(t, "logs")
	mustSee(t, p, 15*time.Second, "daemon ready")
	// A new daemon line must stream while forged logs holds the file open.
	if err := actions.LockSensitive(s.paths); err != nil {
		t.Fatalf("lock: %v", err)
	}
	mustSee(t, p, 15*time.Second, "reason=manual_lock")
	quitTUI(t, p)
}

func (s *scenario) ssh(t *testing.T) (string, error) {
	t.Helper()
	return h.run(t, h.home, "ssh",
		"-p", s.server.Port(),
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"e2e@127.0.0.1", "hello")
}

func (s *scenario) sshEventually(t *testing.T, timeout time.Duration) (string, error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		out, err := s.ssh(t)
		if err == nil && strings.Contains(out, "forged-e2e-ok") {
			return out, nil
		}
		if time.Now().After(deadline) {
			return out, err
		}
		time.Sleep(time.Second)
	}
}

func assertServiceOwned(t *testing.T, s *scenario) {
	t.Helper()
	status, err := daemon.InspectService(s.paths)
	if err != nil || !status.Installed || !status.ConfigValid || !status.Running || !status.PIDKnown {
		t.Fatalf("service status: %+v err=%v", status, err)
	}
	if pid := daemonPID(t, s); pid != status.PID {
		t.Fatalf("pipe served by pid %d, task reports %d", pid, status.PID)
	}
	if err := daemon.RequireServiceOwnership(s.paths); err != nil {
		t.Fatalf("service ownership: %v", err)
	}
}

func daemonPID(t *testing.T, s *scenario) int {
	t.Helper()
	pid, err := platform.PipeServerProcessID(s.paths.CtlSocket())
	if err != nil {
		t.Fatalf("pipe server pid: %v", err)
	}
	return pid
}

func waitPipes(t *testing.T, s *scenario, alive bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if platform.IsSocketAlive(s.paths.CtlSocket()) == alive && platform.IsSocketAlive(s.paths.AgentSocket()) == alive {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("pipes alive != %v after 20s", alive)
}

func waitUnlocked(t *testing.T, s *scenario, unlocked bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last actions.RuntimeStatus
	for time.Now().Before(deadline) {
		status, err := actions.LoadRuntimeStatus(s.paths)
		if err == nil && status.Unlocked == unlocked {
			return
		}
		last = status
		time.Sleep(200 * time.Millisecond)
	}
	b, _ := json.Marshal(last)
	t.Fatalf("session unlocked != %v after 15s: %s", unlocked, b)
}

func mustSee(t *testing.T, p *ptyProcess, timeout time.Duration, needles ...string) {
	t.Helper()
	if text, err := p.WaitFor(timeout, needles...); err != nil {
		t.Fatalf("%v\n---- screen ----\n%s", err, text)
	}
}

func quitTUI(t *testing.T, p *ptyProcess) {
	t.Helper()
	_ = p.Send("\x03")
	if code, err := p.Wait(10 * time.Second); err != nil {
		t.Errorf("TUI did not exit on Ctrl-C: %v\n%s", err, p.Text())
	} else if code != 0 {
		t.Logf("TUI exit code %d", code)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func readFileQuiet(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
