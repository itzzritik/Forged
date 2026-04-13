//go:build linux

package sensitiveauth

func NewLockWatcher() LockWatcher {
	return noopLockWatcher{}
}
