//go:build !windows

package sync

import "os"

func replaceSyncStateFile(source, target string) error {
	return os.Rename(source, target)
}
