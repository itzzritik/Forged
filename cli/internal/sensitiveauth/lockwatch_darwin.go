//go:build darwin

package sensitiveauth

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

type darwinLockWatcher struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewLockWatcher() LockWatcher {
	return &darwinLockWatcher{}
}

func (w *darwinLockWatcher) Start(onLock func()) error {
	if onLock == nil {
		return nil
	}

	notifyutilPath, err := exec.LookPath("notifyutil")
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})

	go w.loop(ctx, notifyutilPath, onLock, w.done)
	return nil
}

func (w *darwinLockWatcher) Stop() error {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	return nil
}

func (w *darwinLockWatcher) loop(ctx context.Context, notifyutilPath string, onLock func(), done chan struct{}) {
	defer close(done)

	for ctx.Err() == nil {
		cmd := exec.CommandContext(
			ctx,
			notifyutilPath,
			"-R",
			"-v",
			"-1", "com.apple.screenIsLocked",
			"-1", "com.apple.screensaver.didstart",
		)
		if _, err := cmd.CombinedOutput(); err == nil {
			if ctx.Err() == nil {
				onLock()
			}
			continue
		}

		if ctx.Err() != nil {
			return
		}

		time.Sleep(time.Second)
	}
}
