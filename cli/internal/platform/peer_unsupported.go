//go:build !linux && !darwin

package platform

import "net"

func AgentPeerPID(conn net.Conn) (int, error) {
	return 0, ErrPeerPIDUnavailable
}
