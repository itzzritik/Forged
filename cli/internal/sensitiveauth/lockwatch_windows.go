//go:build windows

package sensitiveauth

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

type windowsLockWatcher struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewLockWatcher() LockWatcher {
	return &windowsLockWatcher{}
}

func (w *windowsLockWatcher) Start(onLock func()) error {
	if onLock == nil {
		return nil
	}

	shell, err := windowsPowerShellPath()
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

	go w.loop(ctx, shell, onLock, w.done)
	return nil
}

func (w *windowsLockWatcher) Stop() error {
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

func (w *windowsLockWatcher) loop(ctx context.Context, shell string, onLock func(), done chan struct{}) {
	defer close(done)

	for ctx.Err() == nil {
		cmd := exec.CommandContext(
			ctx,
			shell,
			"-NoProfile",
			"-ExecutionPolicy",
			"Bypass",
			"-EncodedCommand",
			encodePowerShellCommand(windowsLockWatchScript),
		)
		if _, err := cmd.CombinedOutput(); err == nil {
			if ctx.Err() == nil {
				onLock()
			}
			continue
		} else {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
				return
			}
		}

		if ctx.Err() != nil {
			return
		}

		time.Sleep(time.Second)
	}
}

const windowsLockWatchScript = `
$source = 'ForgedSessionLock'

try {
  Register-WmiEvent -Class Win32_SessionChangeEvent -SourceIdentifier $source | Out-Null
} catch {
  exit 2
}

try {
  while ($true) {
    $event = Wait-Event -SourceIdentifier $source
    if ($null -eq $event) {
      continue
    }
    try {
      if ($event.SourceEventArgs.NewEvent.Reason -eq 7) {
        Write-Output 'LOCK'
        exit 0
      }
    } finally {
      Remove-Event -EventIdentifier $event.EventIdentifier -ErrorAction SilentlyContinue | Out-Null
    }
  }
} finally {
  Get-EventSubscriber -SourceIdentifier $source -ErrorAction SilentlyContinue | Unregister-Event -Force -ErrorAction SilentlyContinue
}
`
