//go:build windows

package platform

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// pipeSecurityDescriptor is the SDDL applied to the daemon's named pipes.
//
// D:P                  = DACL is protected (no inheritance from parent)
// (A;;GA;;;OW)         = allow GENERIC_ALL to the creator-owner (the user
//                        running the daemon).
// (A;;GA;;;SY)         = allow GENERIC_ALL to LocalSystem so platform
//                        services can interact when needed.
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
