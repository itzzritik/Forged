//go:build darwin

package platform

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func AgentPeerPID(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, ErrPeerPIDUnavailable
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("syscall connection: %w", err)
	}

	var pid int
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		pid, controlErr = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEEREPID)
	}); err != nil {
		return 0, err
	}
	if controlErr != nil {
		return 0, controlErr
	}
	if pid <= 0 {
		return 0, ErrPeerPIDUnavailable
	}
	return pid, nil
}

func PeerCredentialsForConn(conn net.Conn) (PeerCredentials, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return PeerCredentials{}, ErrPeerIdentityUnavailable
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return PeerCredentials{}, fmt.Errorf("syscall connection: %w", err)
	}

	var peer PeerCredentials
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		pid, err := unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEEREPID)
		if err != nil {
			controlErr = err
			return
		}
		cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			controlErr = err
			return
		}
		peer = PeerCredentials{PID: pid, UID: cred.Uid}
	}); err != nil {
		return PeerCredentials{}, err
	}
	if controlErr != nil {
		return PeerCredentials{}, controlErr
	}
	if peer.PID <= 0 {
		return PeerCredentials{}, ErrPeerPIDUnavailable
	}
	return peer, nil
}

func ControlPeerCredentialsAvailable() bool {
	return true
}

func VerifyCurrentUserPeer(conn net.Conn) error {
	peer, err := PeerCredentialsForConn(conn)
	if err != nil {
		return err
	}
	if peer.UID != uint32(os.Geteuid()) {
		return fmt.Errorf("%w: peer uid %d", ErrPeerIdentityMismatch, peer.UID)
	}
	return nil
}

func VerifyDaemonPeer(conn net.Conn, expectedPID int) error {
	peer, err := PeerCredentialsForConn(conn)
	if err != nil {
		return err
	}
	if peer.UID != uint32(os.Geteuid()) {
		return fmt.Errorf("%w: daemon uid %d", ErrPeerIdentityMismatch, peer.UID)
	}
	if expectedPID <= 0 || peer.PID != expectedPID {
		return fmt.Errorf("%w: daemon pid %d", ErrPeerIdentityMismatch, peer.PID)
	}
	return nil
}
