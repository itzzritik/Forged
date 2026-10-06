package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/platform"
)

// Forged runs from one per-user folder rather than from wherever the CLI was
// installed: npm puts it under the active Node version, so switching Node,
// reinstalling or uninstalling would break the service, git signing and SSH
// hooks that point at it.

var installedNames = []string{"forged", "forged-sign", "forged-auth"}

// InstallDir is the per-user folder Forged's binaries are installed into.
func InstallDir() string {
	switch runtime.GOOS {
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Programs", "Forged")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "Forged")
		}
	default:
		if data := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(data) {
			return filepath.Join(data, "forged")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "forged")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".forged-install")
	}
	return ""
}

// InstalledBinary is the fixed path of forged, forged-sign or forged-auth.
func InstalledBinary(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(InstallDir(), "bin", name)
}

// InInstallDir reports whether path is inside InstallDir.
func InInstallDir(path string) bool {
	root := InstallDir()
	if root == "" || strings.TrimSpace(path) == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// InstallBinaries copies source (a forged binary) and the forged-sign and
// forged-auth beside it into InstallDir/bin, rewriting only files that
// changed, and records source so the daemon can follow its upgrades.
func InstallBinaries(source string) error {
	root := InstallDir()
	if root == "" {
		return errors.New("no per-user install folder")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	if InInstallDir(source) {
		return nil
	}
	if strings.TrimSuffix(filepath.Base(source), ".exe") != "forged" {
		return fmt.Errorf("%s is not a forged binary", source)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return err
	}
	unlock, err := lockInstallDir(root)
	if err != nil {
		return fmt.Errorf("locking install folder: %w", err)
	}
	defer unlock()

	for _, name := range installedNames {
		from := source
		if name != "forged" {
			from = filepath.Join(filepath.Dir(source), filepath.Base(InstalledBinary(name)))
		}
		data, err := os.ReadFile(from)
		if errors.Is(err, os.ErrNotExist) && name != "forged" {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if err := replaceBinary(InstalledBinary(name), data); err != nil {
			return fmt.Errorf("installing %s: %w", name, err)
		}
	}
	return os.WriteFile(installSourceFile(), []byte(source), 0o600)
}

// recordedSource is the binary an installed copy at exe came from, if any.
// It resolves from exe rather than InstallDir: the daemon's environment
// (e.g. systemd without XDG_DATA_HOME) can differ from the installing CLI's.
func recordedSource(exe string) string {
	bin := filepath.Dir(exe)
	if filepath.Base(bin) != "bin" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(bin), "source"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func installSourceFile() string {
	return filepath.Join(InstallDir(), "source")
}

func lockInstallDir(root string) (func(), error) {
	file, err := os.OpenFile(filepath.Join(root, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := platform.LockFileWait(file); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = platform.UnlockFile(file)
		_ = file.Close()
	}, nil
}

// replaceBinary swaps in a new file by rename, so a running copy keeps its
// old image. Windows refuses to replace a running exe but lets it be renamed
// aside; those leftovers are removed once nothing runs them.
func replaceBinary(target string, data []byte) error {
	if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
		return nil
	}
	dir := filepath.Dir(target)
	if runtime.GOOS == "windows" {
		leftovers, _ := filepath.Glob(filepath.Join(dir, "*.old-*"))
		for _, old := range leftovers {
			_ = os.Remove(old)
		}
	}
	tmp, err := os.CreateTemp(dir, ".install-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(target); err == nil {
			aside := target + ".old-" + strconv.FormatInt(time.Now().UnixNano(), 36)
			if err := os.Rename(target, aside); err != nil {
				return err
			}
		}
	}
	return os.Rename(tmp.Name(), target)
}
