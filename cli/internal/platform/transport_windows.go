//go:build windows

package platform

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// pipeSecurityDescriptor is the SDDL applied to the daemon's named pipes.
//
// D:P protects the DACL from inheritance.
// (A;;GA;;;OW) grants GENERIC_ALL to the user running the daemon.
// (A;;GA;;;SY) grants GENERIC_ALL to LocalSystem for platform services.
//
// We intentionally do NOT grant BUILTIN\Administrators here, so an
// administrator on a shared Windows machine cannot connect to another
// user's daemon. Root/admin can still take ownership through other Win32
// primitives — see SEC-DAEMON-004 — but the daemon's intent is owner-only.
const pipeSecurityDescriptor = "D:P(A;;GA;;;OW)(A;;GA;;;SY)"

// Listen binds the daemon's agent / ctl pipe. addr must be a full named-pipe
// path (e.g. \\.\pipe\forged-agent).
func Listen(addr string) (net.Listener, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
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
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return winio.DialPipe(addr, &timeout)
}

func DialContext(ctx context.Context, addr string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, addr)
}
