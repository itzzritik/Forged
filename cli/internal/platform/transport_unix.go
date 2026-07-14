//go:build !windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

func CurrentUserPipeIdentityError() error {
	return nil
}

// Listen binds the daemon's agent.sock / ctl.sock as a Unix-domain socket.
// Stale paths must be removed before this call. The returned listener removes
// only the socket inode that it originally bound.
func Listen(addr string) (net.Listener, error) {
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("Listening on %s: %w", addr, err)
	}
	unixListener, ok := ln.(*net.UnixListener)
	if !ok {
		ln.Close()
		return nil, fmt.Errorf("Listening on %s: unexpected listener type %T", addr, ln)
	}
	unixListener.SetUnlinkOnClose(false)

	bound, err := os.Lstat(addr)
	if err != nil {
		unixListener.Close()
		return nil, fmt.Errorf("Inspecting socket %s: %w", addr, err)
	}
	owned := &ownedUnixListener{UnixListener: unixListener, path: addr, bound: bound}
	if err := os.Chmod(addr, 0o600); err != nil {
		owned.Close()
		return nil, fmt.Errorf("Setting socket permissions: %w", err)
	}
	return owned, nil
}

type ownedUnixListener struct {
	*net.UnixListener
	path  string
	bound os.FileInfo
	once  sync.Once
	err   error
}

func (l *ownedUnixListener) Close() error {
	l.once.Do(func() {
		closeErr := l.UnixListener.Close()
		current, statErr := os.Lstat(l.path)
		switch {
		case os.IsNotExist(statErr):
			statErr = nil
		case statErr == nil && os.SameFile(l.bound, current):
			statErr = os.Remove(l.path)
		case statErr == nil:
			statErr = nil
		}
		l.err = errors.Join(closeErr, statErr)
	})
	return l.err
}

// Dial opens a client connection to the daemon. timeout==0 means use a
// short default so callers don't hang on a dead daemon.
func Dial(addr string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return net.DialTimeout("unix", addr, timeout)
}

func DialContext(ctx context.Context, addr string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "unix", addr)
}
