//go:build !linux && !darwin && !windows

package platform

import "net"

func AgentPeerPID(conn net.Conn) (int, error) {
	return 0, ErrPeerPIDUnavailable
}
