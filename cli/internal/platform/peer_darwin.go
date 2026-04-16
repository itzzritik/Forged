//go:build darwin

package platform

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func AgentPeerPID(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, ErrPeerPIDUnavailable
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("syscall conn: %w", err)
	}

	var pid int
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		value, err := unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEEREPID)
		if err != nil {
			controlErr = err
			return
		}
		pid = value
	}); err != nil {
		return 0, err
	}
	if controlErr != nil {
		return 0, controlErr
	}
	if pid == 0 {
		return 0, ErrPeerPIDUnavailable
	}
	return pid, nil
}
