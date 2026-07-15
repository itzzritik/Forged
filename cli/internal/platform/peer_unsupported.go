//go:build !linux && !darwin

package platform

import "net"

func AgentPeerPID(conn net.Conn) (int, error) {
	return 0, ErrPeerPIDUnavailable
}

func PeerCredentialsForConn(conn net.Conn) (PeerCredentials, error) {
	return PeerCredentials{}, ErrPeerIdentityUnavailable
}

func ControlPeerCredentialsAvailable() bool {
	return false
}

func VerifyCurrentUserPeer(conn net.Conn) error {
	return ErrPeerIdentityUnavailable
}

func VerifyDaemonPeer(conn net.Conn, expectedPID int) error {
	return ErrPeerIdentityUnavailable
}
