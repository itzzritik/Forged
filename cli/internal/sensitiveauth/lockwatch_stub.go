//go:build !darwin && !linux && !windows

package sensitiveauth

func NewLockWatcher() LockWatcher {
	return noopLockWatcher{}
}
