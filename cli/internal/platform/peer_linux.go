//go:build linux

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
		cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			controlErr = err
			return
		}
		pid = int(cred.Pid)
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
