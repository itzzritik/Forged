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

// TestNPMInstallFlow installs the npm wrapper and the win32-x64 package the
// way a release lays them out, runs first launch through the forged.cmd
// shim, then upgrades over the running daemon. Opt-in (needs npm); run it
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

	v1 := npmPackages(t, "0.0.0-e2e.1", h.bin)
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
	v2 := npmPackages(t, "0.0.0-e2e.2", nextBin)
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

// npmPackages packs the win32-x64 platform package and the wrapper, with the
// wrapper's optional dependency pointing at the local platform tarball so
// npm installs and orders it exactly like a published dependency.
func npmPackages(t *testing.T, version, bin string) string {
	t.Helper()
	work := filepath.Join(h.root, "npm-"+version)
	platform := filepath.Join(work, "platform")
	wrapper := filepath.Join(work, "cli")
	repo := filepath.Dir(moduleRoot())
	for _, dir := range []string{filepath.Join(platform, "bin"), filepath.Join(wrapper, "bin")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"forged.exe", "forged-sign.exe", "forged-auth.exe"} {
		if err := copyFile(filepath.Join(bin, name), filepath.Join(platform, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	template, err := os.ReadFile(filepath.Join(repo, "npm", "platform", "package.template.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := strings.NewReplacer(
		"__NAME__", "@getforged/cli-win32-x64", "__VERSION__", version, "__OS__", "windows",
		"__ARCH__", "amd64", "__NPM_OS__", "win32", "__NPM_CPU__", "x64",
	).Replace(string(template))
	if err := os.WriteFile(filepath.Join(platform, "package.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	platformTgz := npmPack(t, platform, work)

	for _, name := range []string{"forged.js"} {
		if err := copyFile(filepath.Join(repo, "npm", "cli", "bin", name), filepath.Join(wrapper, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(repo, "npm", "cli", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg map[string]any
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	pkg["version"] = version
	pkg["optionalDependencies"] = map[string]string{"@getforged/cli-win32-x64": "file:" + filepath.ToSlash(platformTgz)}
	raw, _ = json.MarshalIndent(pkg, "", "  ")
	if err := os.WriteFile(filepath.Join(wrapper, "package.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return npmPack(t, wrapper, work)
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
