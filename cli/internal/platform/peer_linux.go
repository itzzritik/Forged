//go:build linux

package platform

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func AgentPeerPID(conn net.Conn) (int, error) {
	peer, err := PeerCredentialsForConn(conn)
	if err != nil {
		return 0, err
	}
	return peer.PID, nil
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
		cred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			controlErr = err
			return
		}
		peer = PeerCredentials{PID: int(cred.Pid), UID: cred.Uid}
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
