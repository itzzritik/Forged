//go:build windows

package config

import (
	"os"

	"golang.org/x/sys/windows"
)

func replacePrivateFile(source, target string, durable bool) error {
	if !durable {
		return os.Rename(source, target)
	}
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
