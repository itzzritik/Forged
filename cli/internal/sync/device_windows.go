//go:build windows

package sync

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func deviceName(hostname string) string { return hostname }

func osVersion() string {
	v := windows.RtlGetVersion()
	// Windows 11 still reports major version 10; build 22000 is the cutoff.
	if v.MajorVersion == 10 && v.BuildNumber >= 22000 {
		return "Windows 11"
	}
	return fmt.Sprintf("Windows %d", v.MajorVersion)
}
