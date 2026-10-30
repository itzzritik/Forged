package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	uninstallCheckInterval = 30 * time.Second
	// An npm update replaces the package folder within seconds.
	uninstallGrace = 10 * time.Minute
)

// watchNPMUninstall removes forged once every npm install it ran from has
// been gone for uninstallGrace.
func (d *Daemon) watchNPMUninstall(executable string) {
	ticker := time.NewTicker(uninstallCheckInterval)
	defer ticker.Stop()
	var clock uninstallClock
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
		}
		if !clock.observe(d.checkNPMInstalls().installed, time.Now()) {
			continue
		}
		check := d.checkNPMInstalls()
		if check.installed {
			clock = uninstallClock{}
			continue
		}
		if !npmSourceGuard(installSource(executable), check.live, LauncherCacheDir()) {
			d.logger.Info("npm install removed, but this copy of forged did not come from npm; leaving it installed")
			return
		}
		d.logger.Info("forged was removed with npm; uninstalling")
		d.uninstall(executable)
		return
	}
}

func (d *Daemon) checkNPMInstalls() npmInstallCheck {
	roots, err := loadNPMInstalls(d.paths)
	if err != nil {
		d.logger.Warn("reading npm install records failed", "error", err)
		return npmInstallCheck{installed: true}
	}
	// Only a folder that is certainly gone counts; an unreadable one is kept.
	check := checkNPMInstalls(roots, func(path string) bool {
		_, err := os.Stat(path)
		return !errors.Is(err, os.ErrNotExist)
	}, holdsForged)
	if len(check.dropped) > 0 {
		_, err := updateNPMInstalls(d.paths, func(current []string) []string {
			return slices.DeleteFunc(current, func(root string) bool { return slices.Contains(check.dropped, root) })
		})
		if err != nil {
			d.logger.Warn("updating npm install records failed", "error", err)
		}
	}
	return check
}

// holdsForged reports whether root has forgedssh or the old @getforged/cli,
// including the copy npm moves aside (.forgedssh-<hash>) while an update
// downloads, which a stalled network can stretch past uninstallGrace.
func holdsForged(root string) bool {
	for _, pkg := range []struct{ dir, name string }{{root, "forgedssh"}, {filepath.Join(root, "@getforged"), "cli"}} {
		entries, err := os.ReadDir(pkg.dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return true
		}
		for _, entry := range entries {
			if name := entry.Name(); name == pkg.name || strings.HasPrefix(name, "."+pkg.name+"-") {
				return true
			}
		}
	}
	return false
}

// installSource is the binary the install folder was copied from.
func installSource(executable string) string {
	if runtime.GOOS != "windows" {
		return recordedSource(executable)
	}
	data, err := os.ReadFile(filepath.Join(InstallDir(), "source"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

type npmInstallCheck struct {
	live, dropped []string
	installed     bool
}

// checkNPMInstalls drops roots whose folder is gone (that Node version or
// package manager was removed) and reports forged installed while any live
// root still holds it, or none are left to judge.
func checkNPMInstalls(roots []string, exists, holds func(string) bool) npmInstallCheck {
	var check npmInstallCheck
	for _, root := range roots {
		if !exists(root) {
			check.dropped = append(check.dropped, root)
			continue
		}
		check.live = append(check.live, root)
		if holds(root) {
			check.installed = true
		}
	}
	if len(check.live) == 0 {
		check.installed = true
	}
	return check
}

type uninstallClock struct {
	since time.Time
}

// observe reports true once forged has been missing for uninstallGrace without a break.
func (c *uninstallClock) observe(installed bool, now time.Time) bool {
	if installed {
		c.since = time.Time{}
		return false
	}
	if c.since.IsZero() {
		c.since = now
	}
	return now.Sub(c.since) >= uninstallGrace
}

// npmSourceGuard reports whether the installed copy came from npm, so a dev
// build or manual install is never removed because of an old record.
func npmSourceGuard(source string, roots []string, cacheDir string) bool {
	for _, root := range slices.Concat(roots, []string{cacheDir}) {
		if inDir(root, source) {
			return true
		}
	}
	return false
}
