//go:build windows

package platform

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// Listen binds the daemon's agent / ctl pipe. addr must be a full named-pipe
// path (e.g. \\.\pipe\<name>).
func Listen(addr string) (net.Listener, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	// Explicit user owner, not OW: an elevated daemon would otherwise be
	// owned by Administrators. Administrators are deliberately not granted.
	user := sid.String()
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "O:" + user + "D:P(A;;GA;;;" + user + ")(A;;GA;;;SY)",
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}
	ln, err := winio.ListenPipe(addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("Listening on pipe %s: %w", addr, err)
	}
	return ln, nil
}

// Dial opens a client connection to the daemon over its named pipe.
func Dial(addr string, timeout time.Duration) (net.Conn, error) {
	if err := CurrentUserPipeIdentityError(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return verifiedPipe(winio.DialPipe(addr, &timeout))
}

func DialContext(ctx context.Context, addr string) (net.Conn, error) {
	if err := CurrentUserPipeIdentityError(); err != nil {
		return nil, err
	}
	return verifiedPipe(winio.DialPipeContext(ctx, addr))
}

func verifiedPipe(conn net.Conn, err error) (net.Conn, error) {
	if err != nil {
		return nil, err
	}
	if err := verifyPipeServerUser(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}
