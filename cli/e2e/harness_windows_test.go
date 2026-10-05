//go:build e2e && windows

package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/platform"
	"golang.org/x/sys/windows"
)

// Pipe names derive from the Windows account, not the profile, so the suite
// refuses to run while any Forged daemon of this account is live.

type harness struct {
	root     string
	home     string
	bin      string
	fakeBin  string
	realAuth string
	schedDir string
	authDir  string
	buildID  string
	env      []string
	keep     bool
}

var h *harness

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	agent, ctl, err := platform.CurrentUserPipePaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: resolving pipe identity:", err)
		return 2
	}
	if platform.IsSocketAlive(agent) || platform.IsSocketAlive(ctl) {
		fmt.Fprintln(os.Stderr, "e2e: a Forged daemon for this Windows account is running; its pipes would be shared with the suite. Stop it first.")
		return 2
	}
	// go test may start the binary with Ctrl+C ignored, which children inherit.
	_, _, _ = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler").Call(0, 0)

	var setupErr error
	h, setupErr = newHarness()
	if setupErr != nil {
		fmt.Fprintln(os.Stderr, "e2e: setup:", setupErr)
		if h != nil {
			h.cleanup()
		}
		return 2
	}
	fmt.Fprintf(os.Stderr, "e2e: root=%s build=%s\n", h.root, h.buildID)
	code := m.Run()
	if code != 0 {
		h.keep = h.keep || os.Getenv("FORGED_E2E_KEEP_ON_FAIL") == "1"
	}
	h.cleanup()
	return code
}

// Fixed so one Defender exclusion covers every run.
func e2eBase() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "ForgedE2E")
}

func newHarness() (*harness, error) {
	if err := os.MkdirAll(e2eBase(), 0o700); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(e2eBase(), "run-")
	if err != nil {
		return nil, err
	}
	suffix := randomHex(4)
	hh := &harness{
		root:     root,
		home:     filepath.Join(root, "fe2e-"+suffix),
		bin:      filepath.Join(root, "bin"),
		fakeBin:  filepath.Join(root, "fakebin"),
		realAuth: filepath.Join(root, "realauth"),
		schedDir: filepath.Join(root, "sched"),
		authDir:  filepath.Join(root, "auth"),
		buildID:  "e2e-" + time.Now().UTC().Format("20060102T150405Z") + "-" + suffix,
		keep:     os.Getenv("FORGED_E2E_KEEP") == "1",
	}
	for _, dir := range []string{hh.home, hh.bin, hh.fakeBin, hh.realAuth, hh.schedDir, hh.authDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return hh, err
		}
	}
	if err := hh.build(); err != nil {
		return hh, err
	}
	hh.env = hh.environment()
	for _, kv := range hh.env {
		k, v, _ := strings.Cut(kv, "=")
		os.Setenv(k, v)
	}
	return hh, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func moduleRoot() string {
	wd, _ := os.Getwd()
	return filepath.Dir(wd)
}

func goBuild(out, pkg, buildID string) error {
	cmd := exec.Command("go", "build", "-tags", "e2e", "-o", out,
		"-ldflags", "-X github.com/itzzritik/forged/cli/internal/buildinfo.ID="+buildID, pkg)
	cmd.Dir = moduleRoot()
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, output)
	}
	return nil
}

func (hh *harness) build() error {
	builds := map[string]string{
		filepath.Join(hh.bin, "forged.exe"):           "./cmd/forged",
		filepath.Join(hh.bin, "forged-sign.exe"):      "./cmd/forged-sign",
		filepath.Join(hh.bin, "forged-auth.exe"):      "./e2e/fakes/fakeauth",
		filepath.Join(hh.realAuth, "forged-auth.exe"): "./cmd/forged-auth",
		filepath.Join(hh.fakeBin, "schtasks.exe"):     "./e2e/fakes/fakesched",
		filepath.Join(hh.fakeBin, "ssh.exe"):          "./e2e/fakes/fakessh",
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(builds))
	for out, pkg := range builds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- goBuild(out, pkg, hh.buildID)
		}()
	}
	wg.Wait()
	close(errs)
	var all []error
	for err := range errs {
		all = append(all, err)
	}
	if err := errors.Join(all...); err != nil {
		return err
	}
	return copyFile(filepath.Join(hh.fakeBin, "schtasks.exe"), filepath.Join(hh.fakeBin, "powershell.exe"))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o700)
}

func (hh *harness) environment() []string {
	drop := map[string]bool{
		"SSH_AUTH_SOCK": true, "USERPROFILE": true, "HOME": true, "APPDATA": true,
		"LOCALAPPDATA": true, "GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true,
		"PATH": true, "WT_SESSION": true, "TERM_PROGRAM": true, "FORGED_ASCII": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if drop[strings.ToUpper(k)] || strings.HasPrefix(k, "=") {
			continue
		}
		env = append(env, kv)
	}
	sys := os.Getenv("SystemRoot")
	path := strings.Join([]string{
		hh.fakeBin,
		hh.bin,
		filepath.Join(sys, "System32", "OpenSSH"),
		filepath.Join(sys, "System32"),
		sys,
		filepath.Join(sys, "System32", "WindowsPowerShell", "v1.0"),
		filepath.Dir(mustLookPath("git")),
		filepath.Dir(mustLookPath("go")),
	}, ";")
	if node, err := exec.LookPath("node"); err == nil {
		path += ";" + filepath.Dir(node)
	}
	return append(env,
		"USERPROFILE="+hh.home,
		"HOME="+hh.home,
		"APPDATA="+filepath.Join(hh.home, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(hh.home, "AppData", "Local"),
		"GIT_CONFIG_GLOBAL="+filepath.Join(hh.home, ".gitconfig"),
		"GIT_CONFIG_NOSYSTEM=1",
		// Never reach the developer's real agent on the default pipe.
		`SSH_AUTH_SOCK=\\.\pipe\forged-e2e-no-agent`,
		"FORGED_E2E_SCHED_DIR="+hh.schedDir,
		"FORGED_E2E_AUTH_DIR="+hh.authDir,
		"PATH="+path,
	)
}

func mustLookPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		panic(err)
	}
	return p
}

func (hh *harness) cleanup() {
	killProcessesUnder(hh.root)
	if hh.keep {
		fmt.Fprintln(os.Stderr, "e2e: keeping", hh.root)
		return
	}
	for i := 0; i < 20; i++ {
		if err := os.RemoveAll(hh.root); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "e2e: could not remove", hh.root)
}

// Covers daemons the fake scheduler detached from the suite.
func killProcessesUnder(dir string) {
	prefix := strings.ToLower(filepath.Clean(dir)) + `\`
	for _, pid := range processesUnder(prefix) {
		if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid); err == nil {
			_ = windows.TerminateProcess(h, 1)
			_, _ = windows.WaitForSingleObject(h, 5000)
			windows.CloseHandle(h)
		}
	}
}

func processesUnder(prefix string) []uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var pids []uint32
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		if image := processImage(entry.ProcessID); strings.HasPrefix(strings.ToLower(image), prefix) {
			pids = append(pids, entry.ProcessID)
		}
	}
	return pids
}

func processImage(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

func (hh *harness) forged(t *testing.T, args ...string) *ptyProcess {
	t.Helper()
	argv := append([]string{filepath.Join(hh.bin, "forged.exe")}, args...)
	p, err := startPTY(argv, hh.env, hh.home, 120, 40)
	if err != nil {
		t.Fatalf("starting forged: %v", err)
	}
	t.Cleanup(p.Kill)
	return p
}

func (hh *harness) run(t *testing.T, dir string, name string, args ...string) (string, error) {
	t.Helper()
	return hh.runEnv(t, nil, dir, name, args...)
}

func (hh *harness) runEnv(t *testing.T, extra []string, dir string, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(append([]string(nil), hh.env...), extra...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (hh *harness) setAuth(t *testing.T, name, value string) {
	t.Helper()
	path := filepath.Join(hh.authDir, name)
	if value == "" {
		_ = os.Remove(path)
		return
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

// createVault walks the first-run TUI to a healthy local vault.
func createVault(t *testing.T, p *ptyProcess) {
	t.Helper()
	mustSee(t, p, 20*time.Second, "Create local vault")
	_ = p.Send("\x1b[B")
	time.Sleep(300 * time.Millisecond)
	_ = p.Send("\r")
	mustSee(t, p, 10*time.Second, "Confirm master password")
	_ = p.Type(e2ePassword)
	_ = p.Send("\r")
	time.Sleep(300 * time.Millisecond)
	_ = p.Type(e2ePassword)
	_ = p.Send("\r")
	mustSee(t, p, 30*time.Second, "DASHBOARD", "System healthy")
	quitTUI(t, p)
}

func stagedDaemonRoot() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Forged", "daemon")
}
