package daemon

import (
	"fmt"
	"os"
	"path/filepath"

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
		if runtimeLockBusy(err) {
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
