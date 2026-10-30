//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var procSendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// EnsureOnPath appends the install folder's bin to the user PATH.
// Appended, so a package manager's forged still wins while it exists.
func EnsureOnPath() (bool, error) {
	bin := filepath.Join(InstallDir(), "bin")
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, fmt.Errorf("opening user environment: %w", err)
	}
	defer key.Close()

	current, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return false, fmt.Errorf("reading user PATH: %w", err)
	}
	for _, dir := range filepath.SplitList(current) {
		if expanded, err := registry.ExpandString(dir); err == nil {
			dir = expanded
		}
		if strings.EqualFold(filepath.Clean(dir), filepath.Clean(bin)) {
			return false, nil
		}
	}
	if current != "" && !strings.HasSuffix(current, ";") {
		current += ";"
	}
	if err := key.SetExpandStringValue("Path", current+bin); err != nil {
		return false, fmt.Errorf("writing user PATH: %w", err)
	}
	broadcastEnvironmentChange()
	return true, nil
}

// RemoveFromPath drops the install folder's bin from the user PATH.
func RemoveFromPath() error {
	bin := filepath.Join(InstallDir(), "bin")
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening user environment: %w", err)
	}
	defer key.Close()

	current, _, err := key.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading user PATH: %w", err)
	}
	var kept []string
	for _, dir := range strings.Split(current, ";") {
		check := dir
		if expanded, err := registry.ExpandString(dir); err == nil {
			check = expanded
		}
		if strings.TrimSpace(check) != "" && strings.EqualFold(filepath.Clean(check), filepath.Clean(bin)) {
			continue
		}
		kept = append(kept, dir)
	}
	next := strings.Join(kept, ";")
	if next == current {
		return nil
	}
	if err := key.SetExpandStringValue("Path", next); err != nil {
		return fmt.Errorf("writing user PATH: %w", err)
	}
	broadcastEnvironmentChange()
	return nil
}

// New terminals started from Explorer only see a PATH change after this broadcast.
func broadcastEnvironmentChange() {
	env, _ := windows.UTF16PtrFromString("Environment")
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	procSendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000, 0)
}
