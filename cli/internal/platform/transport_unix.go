//go:build !windows

package platform

import (
	"fmt"
	"net"
	"os"
	"time"
)

// Listen binds the daemon's agent.sock / ctl.sock as a Unix-domain socket.
// Removes any stale inode at the same path first; sets 0o600 perms before
// returning so the brief umask-default window after net.Listen is closed.
func Listen(addr string) (net.Listener, error) {
	os.Remove(addr)
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("Listening on %s: %w", addr, err)
	}
	if err := os.Chmod(addr, 0o600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("Setting socket permissions: %w", err)
	}
	return ln, nil
}

// Dial opens a client connection to the daemon. timeout==0 means use a
// short default so callers don't hang on a dead daemon.
func Dial(addr string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return net.DialTimeout("unix", addr, timeout)
}
