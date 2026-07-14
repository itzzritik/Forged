package platform

import "runtime"

// SSHRoutingSupported reports whether route-scoped SSH key access is safe on this platform.
func SSHRoutingSupported() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	default:
		return false
	}
}
