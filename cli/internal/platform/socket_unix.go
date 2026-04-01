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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if IsSocketAlive(path) {
		return fmt.Errorf("socket %s is in use by another process", path)
	}
	return os.Remove(path)
}
