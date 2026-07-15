//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

type runtimeLock struct {
	file *os.File
}

func acquireRuntimeLock(paths config.Paths) (*runtimeLock, error) {
	if err := ensureRuntimeDirectory(paths.RuntimeDir); err != nil {
		return nil, err
	}
	path := filepath.Join(paths.RuntimeDir, "daemon.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("Opening daemon lock: %w", err)
	}
	if err := platform.LockFile(file); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("Daemon already running")
		}
		return nil, fmt.Errorf("Locking daemon runtime: %w", err)
	}
	return &runtimeLock{file: file}, nil
}

func ensureRuntimeDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("creating runtime directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspecting runtime directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("runtime path is not a directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("runtime directory is not owned by the current user")
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("securing runtime directory: %w", err)
		}
	}
	return nil
}

func (l *runtimeLock) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = platform.UnlockFile(l.file)
	_ = l.file.Close()
	l.file = nil
}
