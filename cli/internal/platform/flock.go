package platform

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrLockBusy means another process kept a lock past the wait; it says nothing about the data.
var ErrLockBusy = errors.New("lock is busy")

// LockBusy reports that another process holds the lock, as opposed to a file system that
// cannot lock at all (NFS without lockd, some WSL mounts).
func LockBusy(err error) bool { return lockBusy(err) }

// LockFileWithin waits at most timeout for an exclusive lock: a holder that is stopped (Ctrl-Z)
// or hung must not freeze a daemon that takes the lock while serving every client.
func LockFileWithin(f *os.File, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := LockFile(f)
		if err == nil || !lockBusy(err) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: another Forged process has held %s for %s", ErrLockBusy, f.Name(), timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
