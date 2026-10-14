//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

// ReplaceFile renames from over to and syncs the directory, so the rename itself survives a crash.
func ReplaceFile(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	return SyncDir(filepath.Dir(to))
}

func SyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
