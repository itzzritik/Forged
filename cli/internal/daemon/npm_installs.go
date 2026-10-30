package daemon

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

// NPMRoot is the outermost node_modules folder in packageDir: the global
// packages folder, also for nested (bridge) and pnpm store paths.
func NPMRoot(packageDir string) (string, bool) {
	clean := filepath.Clean(packageDir)
	volume := filepath.VolumeName(clean)
	parts := strings.Split(clean[len(volume):], string(filepath.Separator))
	for i, part := range parts {
		if part == "node_modules" {
			return volume + strings.Join(parts[:i+1], string(filepath.Separator)), true
		}
	}
	return "", false
}

// RecordNPMInstall remembers the global packages folder the npm launcher ran from.
func RecordNPMInstall(paths config.Paths, packageDir string) error {
	root, ok := NPMRoot(packageDir)
	if !ok {
		return nil
	}
	_, err := updateNPMInstalls(paths, func(roots []string) []string {
		if slices.Contains(roots, root) {
			return roots
		}
		return append(roots, root)
	})
	return err
}

func loadNPMInstalls(paths config.Paths) ([]string, error) {
	return updateNPMInstalls(paths, nil)
}

// updateNPMInstalls applies change under the records file's lock, so a
// launch recording a new folder is never lost to a concurrent rewrite.
// A nil change only reads; a missing file reads as no records.
func updateNPMInstalls(paths config.Paths, change func([]string) []string) ([]string, error) {
	flag := os.O_RDWR
	if change != nil {
		if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
			return nil, fmt.Errorf("creating config folder: %w", err)
		}
		flag |= os.O_CREATE
	}
	file, err := os.OpenFile(paths.NPMInstallsFile(), flag, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening npm install records: %w", err)
	}
	defer file.Close()
	if err := platform.LockFileWait(file); err != nil {
		return nil, fmt.Errorf("locking npm install records: %w", err)
	}
	defer platform.UnlockFile(file)

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading npm install records: %w", err)
	}
	var roots []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			roots = append(roots, line)
		}
	}
	if change == nil {
		return roots, nil
	}
	next := change(slices.Clone(roots))
	if slices.Equal(next, roots) {
		return next, nil
	}
	data = nil
	for _, root := range next {
		data = append(data, root+"\n"...)
	}
	if err := file.Truncate(0); err != nil {
		return nil, fmt.Errorf("writing npm install records: %w", err)
	}
	if _, err := file.WriteAt(data, 0); err != nil {
		return nil, fmt.Errorf("writing npm install records: %w", err)
	}
	return next, nil
}

// LauncherCacheDir is where the npm launcher keeps binaries when its own
// package folder is not writable; npm/bin/forged.js must agree.
func LauncherCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
		if runtime.GOOS == "windows" {
			dir = filepath.Join(home, "AppData", "Local")
		}
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "forged", "cache")
	}
	return filepath.Join(dir, "forged")
}
