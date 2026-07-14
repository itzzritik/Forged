//go:build !windows

package platform

import (
	"fmt"
	"net"
	"os"
	"time"
)

func IsSocketAlive(path string) bool {
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func CleanStaleSocket(path string) error {
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Inspecting socket %s: %w", path, err)
	}
	if before.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("Refusing to remove non-socket path %s", path)
	}
	if IsSocketAlive(path) {
		return fmt.Errorf("Socket %s is in use by another process", path)
	}
	after, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Rechecking socket %s: %w", path, err)
	}
	if !os.SameFile(before, after) {
		return fmt.Errorf("Socket %s changed while checking it", path)
	}
	return os.Remove(path)
}
