//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func runtimeLockBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
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
