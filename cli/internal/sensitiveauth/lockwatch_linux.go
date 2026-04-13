//go:build linux

package sensitiveauth

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type linuxLockWatcher struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewLockWatcher() LockWatcher {
	return &linuxLockWatcher{}
}

func (w *linuxLockWatcher) Start(onLock func()) error {
	if onLock == nil {
		return nil
	}

	path, err := exec.LookPath("gdbus")
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

	go w.loop(ctx, path, onLock, w.done)
	return nil
}

func (w *linuxLockWatcher) Stop() error {
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

func (w *linuxLockWatcher) loop(ctx context.Context, gdbusPath string, onLock func(), done chan struct{}) {
	defer close(done)

	for ctx.Err() == nil {
		cmd := exec.CommandContext(
			ctx,
			gdbusPath,
			"monitor",
			"--session",
			"--dest", "org.freedesktop.ScreenSaver",
			"--object-path", "/org/freedesktop/ScreenSaver",
		)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if err := cmd.Start(); err != nil {
			time.Sleep(time.Second)
			continue
		}

		triggered := false
		doneReading := make(chan struct{})
		go func() {
			defer close(doneReading)
			scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
			for scanner.Scan() {
				line := strings.ToLower(scanner.Text())
				if strings.Contains(line, "activechanged") && strings.Contains(line, "true") {
					triggered = true
					onLock()
				}
			}
		}()

		_ = cmd.Wait()
		<-doneReading

		if ctx.Err() != nil {
			return
		}
		if !triggered {
			time.Sleep(time.Second)
		}
	}
}
