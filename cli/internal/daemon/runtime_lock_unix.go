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
	if err := os.MkdirAll(paths.RuntimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("Creating runtime directory: %w", err)
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

func (l *runtimeLock) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = platform.UnlockFile(l.file)
	_ = l.file.Close()
	l.file = nil
}
