//go:build !windows

package platform

import (
	"errors"
	"os"
	"os/exec"
)

var errWindowsOnly = errors.New("only available on Windows")

func DetachOwnedConsole() {}

func IsMSYSTerminal(*os.File) bool { return false }

func HideChildConsole(*exec.Cmd) {}

func PipeServerProcessID(string) (int, error) { return 0, ErrPeerPIDUnavailable }

func ClipboardWriteText([]byte, bool) error { return errWindowsOnly }

func ClipboardReadText() ([]byte, error) { return nil, errWindowsOnly }
