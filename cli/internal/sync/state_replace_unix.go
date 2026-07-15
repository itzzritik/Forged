//go:build !windows

package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func replaceSyncStateFile(source, target string) error {
	return os.Rename(source, target)
}

func moveSyncStateFileNoReplace(source, destination string) error {
	if err := os.Link(source, destination); errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: destination sync state exists", ErrStateRecoveryRequired)
	} else if err != nil {
		return fmt.Errorf("linking sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(source)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("removing moved sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(source)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	return nil
}
