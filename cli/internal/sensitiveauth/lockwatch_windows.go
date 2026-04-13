//go:build windows

package sensitiveauth

func NewLockWatcher() LockWatcher {
	return noopLockWatcher{}
}
