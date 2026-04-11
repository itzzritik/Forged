package sync

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

type Bus struct {
	engine    *Engine
	logger    *slog.Logger
	mu        sync.Mutex
	dirty     bool
	timer     *time.Timer
	debounce  time.Duration
	dirtyFile string
}

func NewBus(engine *Engine, logger *slog.Logger, dirtyFile string) *Bus {
	return &Bus{
		engine:    engine,
		logger:    logger,
		debounce:  5 * time.Second,
		dirtyFile: dirtyFile,
	}
}

func (b *Bus) MarkDirty(reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.dirty = true
	b.logger.Debug("vault marked dirty", "reason", reason)

	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.debounce, func() {
		b.flush()
	})
}

func (b *Bus) RequestPull(reason string) {
	b.logger.Debug("pull requested", "reason", reason)
	if err := b.engine.Pull(); err != nil {
		b.logger.Debug("pull failed", "error", err)
	}
}

func (b *Bus) flush() {
	b.mu.Lock()
	if !b.dirty {
		b.mu.Unlock()
		return
	}
	b.dirty = false
	b.mu.Unlock()

	if err := b.engine.Pull(); err != nil {
		b.logger.Debug("pre-push pull failed", "error", err)
	}

	if err := b.engine.push(); err != nil {
		b.logger.Debug("push failed, will retry", "error", err)
		b.mu.Lock()
		b.dirty = true
		b.mu.Unlock()
		b.engine.retryWithBackoff(b.engine.push)
	} else {
		b.removeDirtyFile()
	}
}

func (b *Bus) PersistDirtyFlag() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.dirty && b.dirtyFile != "" {
		os.WriteFile(b.dirtyFile, []byte("1"), 0600)
	}
}

func (b *Bus) CheckDirtyFlag() {
	if b.dirtyFile == "" {
		return
	}
	if _, err := os.Stat(b.dirtyFile); err == nil {
		os.Remove(b.dirtyFile)
		b.MarkDirty("persisted_dirty_flag")
	}
}

func (b *Bus) removeDirtyFile() {
	if b.dirtyFile != "" {
		os.Remove(b.dirtyFile)
	}
}
