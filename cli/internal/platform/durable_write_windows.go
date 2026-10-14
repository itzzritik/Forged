//go:build windows

package platform

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// ReplaceFile replaces to with from, write-through. Another process reading to (os.ReadFile opens
// without FILE_SHARE_DELETE, antivirus, backup) makes the replace fail briefly, so it retries.
func ReplaceFile(from, to string) error {
	src, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		err = windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err == nil || attempt == 10 || !(errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)) {
			return err
		}
		time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
	}
}

// SyncDir is a no-op: Windows has no portable directory flush; MOVEFILE_WRITE_THROUGH covers renames.
func SyncDir(string) error { return nil }
