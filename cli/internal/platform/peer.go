package platform

import "errors"

var (
	ErrPeerPIDUnavailable      = errors.New("peer PID unavailable")
	ErrPeerIdentityUnavailable = errors.New("peer identity unavailable")
	ErrPeerIdentityMismatch    = errors.New("peer identity mismatch")
)

type PeerCredentials struct {
	PID int
	UID uint32
}
