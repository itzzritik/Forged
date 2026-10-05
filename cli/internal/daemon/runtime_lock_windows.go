//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func ensureRuntimeDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("creating runtime directory: %w", err)
	}
	return nil
}

func runtimeLockBusy(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
