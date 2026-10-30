//go:build e2e && windows

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

// TestNPMInstallFlow installs the forgedssh package, runs first launch
// through the forged.cmd shim, then upgrades over the running daemon. Opt-in (needs npm); run it
// alone because it needs a fresh profile:
//
//	FORGED_E2E_NPM=1 go test -tags e2e ./e2e -run TestNPMInstallFlow -v
func TestNPMInstallFlow(t *testing.T) {
	if os.Getenv("FORGED_E2E_NPM") != "1" {
		t.Skip("set FORGED_E2E_NPM=1 to run the npm install flow")
	}
	paths := config.DefaultPaths()
	if fileExists(paths.VaultFile()) {
		t.Skip("needs a fresh profile; run TestNPMInstallFlow alone")
	}
	prefix := filepath.Join(h.root, "npm-prefix")
	shim := filepath.Join(prefix, "forged.cmd")

	v1 := npmPackage(t, "0.0.0-e2e.1", h.bin)
	if out, err := h.run(t, h.root, "npm", "install", "-g", "--prefix", prefix, "--no-audit", "--no-fund", v1); err != nil {
		t.Fatalf("npm install: %v\n%s", err, out)
	}
	if out, err := h.run(t, h.root, shim, "version"); err != nil || !strings.HasPrefix(out, "forged ") {
		t.Fatalf("forged.cmd version: %v\n%s", err, out)
	}

	p, err := startPTY([]string{filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/d", "/c", shim}, h.env, h.home, 120, 40)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Kill)
	createVault(t, p)

	s := &scenario{paths: paths}
	if image := processImage(uint32(daemonPID(t, s))); !strings.HasPrefix(image, stagedDaemonRoot()+`\`) {
		t.Fatalf("daemon runs %s; want a staged copy, not the npm install", image)
	}

	// npm must replace its files under the live daemon without install
	// scripts, and the next normal launch must move the daemon to the new build.
	nextBin := filepath.Join(h.root, "bin-next")
	nextBuild := h.buildID + "-npm2"
	if err := goBuild(filepath.Join(nextBin, "forged.exe"), "./cmd/forged", nextBuild); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"forged-sign.exe", "forged-auth.exe"} {
		if err := copyFile(filepath.Join(h.bin, name), filepath.Join(nextBin, name)); err != nil {
			t.Fatal(err)
		}
	}
	v2 := npmPackage(t, "0.0.0-e2e.2", nextBin)
	out, err := h.run(t, h.root, "npm", "install", "-g", "--prefix", prefix, "--no-audit", "--no-fund", v2)
	if err != nil || strings.Contains(out, "warn") {
		t.Fatalf("npm upgrade while the daemon runs: %v\n%s", err, out)
	}

	p, err = startPTY([]string{filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"), "/d", "/c", shim}, h.env, h.home, 120, 40)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Kill)
	mustSee(t, p, 40*time.Second, "Overview", "Healthy")
	quitTUI(t, p)
	if got, err := daemon.RunningBuildID(paths); err != nil || got != nextBuild {
		t.Fatalf("after npm upgrade and relaunch the daemon runs build %q (err=%v), want %q", got, err, nextBuild)
	}
	assertServiceOwned(t, s)
}

// npmPackage packs forgedssh with the binaries and their .version marker
// already in bin/, so the launcher runs them without downloading.
func npmPackage(t *testing.T, version, bin string) string {
	t.Helper()
	work := filepath.Join(h.root, "npm-"+version)
	pkgDir := filepath.Join(work, "forgedssh")
	repo := filepath.Dir(moduleRoot())
	if err := os.MkdirAll(filepath.Join(pkgDir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"forged.exe", "forged-sign.exe", "forged-auth.exe"} {
		if err := copyFile(filepath.Join(bin, name), filepath.Join(pkgDir, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyFile(filepath.Join(repo, "npm", "bin", "forged.js"), filepath.Join(pkgDir, "bin", "forged.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bin", ".version"), []byte(version), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repo, "npm", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	pkg["version"] = version
	unused := map[string]string{"url": "https://127.0.0.1:9/unused.zip", "sha256": "0"}
	pkg["forged"] = map[string]any{"archives": map[string]any{"win32-x64": unused, "win32-arm64": unused}}
	raw, _ = json.MarshalIndent(pkg, "", "  ")
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return npmPack(t, pkgDir, work)
}

func npmPack(t *testing.T, dir, dest string) string {
	t.Helper()
	out, err := h.run(t, dir, "npm", "pack", "--pack-destination", dest, "--silent")
	if err != nil {
		t.Fatalf("npm pack %s: %v\n%s", dir, err, out)
	}
	lines := strings.Fields(strings.TrimSpace(out))
	return filepath.Join(dest, lines[len(lines)-1])
}
