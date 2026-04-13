//go:build darwin

package sensitiveauth

func NewLockWatcher() LockWatcher {
	return noopLockWatcher{}
}
