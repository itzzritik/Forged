package sync

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type EngineRuntime interface {
	PushCurrent(ctx context.Context, state *SyncState) error
	PullLatest(ctx context.Context, state *SyncState) (vault.VaultData, PullResult, error)
}

type mergeRetryRuntime interface {
	MergeAndRetry(ctx context.Context, state *SyncState) error
}

type remoteStatusRuntime interface {
	RemoteStatus(ctx context.Context, state *SyncState) (StatusResult, error)
}

type linkRuntime interface {
	ReconcileOnLink(ctx context.Context, state *SyncState, userID, serverURL string) error
}

type BusConfig struct {
	MutationDebounce     time.Duration
	ForegroundFreshness  time.Duration
	AgentFreshnessWindow time.Duration
	RemoteCheckInterval  time.Duration
	RemoteCheckTimeout   time.Duration
	RetryBackoff         []time.Duration
	DirtyFlagPath        string
	StateStore           *StateStore
}

type Bus struct {
	engine EngineRuntime
	state  *SyncState
	logger *slog.Logger
	cfg    BusConfig

	mu               sync.Mutex
	timer            *time.Timer
	retryTimer       *time.Timer
	syncing          bool
	syncDone         chan struct{}
	queuedPush       bool
	queuedRefresh    bool
	lastAgentRefresh time.Time
	retryIndex       int
	mutationVersion  uint64
	stopped          bool
	stopOnce         sync.Once
	stopCh           chan struct{}
	stopCtx          context.Context
	cancelStop       context.CancelFunc
	active           sync.WaitGroup
}

func DefaultBusConfig() BusConfig {
	return BusConfig{
		MutationDebounce:     500 * time.Millisecond,
		ForegroundFreshness:  5 * time.Second,
		AgentFreshnessWindow: 30 * time.Second,
		RemoteCheckInterval:  2 * time.Minute,
		RemoteCheckTimeout:   3 * time.Second,
		RetryBackoff: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			5 * time.Second,
			10 * time.Second,
			30 * time.Second,
			1 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
	}
}

func NewBus(engine EngineRuntime, state *SyncState, logger *slog.Logger, cfg BusConfig) *Bus {
	defaults := DefaultBusConfig()
	if cfg.MutationDebounce <= 0 {
		cfg.MutationDebounce = defaults.MutationDebounce
	}
	if cfg.ForegroundFreshness <= 0 {
		cfg.ForegroundFreshness = defaults.ForegroundFreshness
	}
	if cfg.AgentFreshnessWindow <= 0 {
		cfg.AgentFreshnessWindow = defaults.AgentFreshnessWindow
	}
	if cfg.RemoteCheckInterval <= 0 {
		cfg.RemoteCheckInterval = defaults.RemoteCheckInterval
	}
	if cfg.RemoteCheckTimeout <= 0 {
		cfg.RemoteCheckTimeout = defaults.RemoteCheckTimeout
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = defaults.RetryBackoff
	}
	if logger == nil {
		logger = slog.Default()
	}
	if state == nil {
		defaultState := DefaultSyncState("")
		state = &defaultState
	}

	stopCtx, cancelStop := context.WithCancel(context.Background())
	bus := &Bus{
		engine:     engine,
		state:      state,
		logger:     logger,
		cfg:        cfg,
		stopCh:     make(chan struct{}),
		stopCtx:    stopCtx,
		cancelStop: cancelStop,
	}
	bus.startBackgroundRefresh()
	return bus
}

func (b *Bus) LocalMutation(reason string) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.mutationVersion++
	b.state.MarkDirty("", time.Time{})
	b.persistLocked()

	if b.retryTimer != nil {
		b.retryTimer.Stop()
		b.retryTimer = nil
	}
	if b.syncing {
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		b.queuedPush = true
		b.mu.Unlock()
		return
	}
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.cfg.MutationDebounce, func() {
		b.enqueuePush("mutation:" + reason)
	})
	b.mu.Unlock()
}

func (b *Bus) ForegroundRead(ctx context.Context, reason string) error {
	for {
		b.mu.Lock()
		if b.stopped {
			b.mu.Unlock()
			return nil
		}
		if b.state.Dirty {
			b.mu.Unlock()
			return nil
		}
		if !b.isStaleLocked(b.cfg.ForegroundFreshness) {
			b.mu.Unlock()
			return nil
		}
		if b.syncing {
			done := b.syncDone
			b.mu.Unlock()
			if err := waitForSync(ctx, done); err != nil {
				return err
			}
			continue
		}
		b.beginSyncLocked()
		b.mu.Unlock()

		err := b.executeRefresh(ctx, reason)
		b.finishPull(err)
		return err
	}
}

func (b *Bus) AgentAccess(reason string) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	if b.state.Dirty {
		b.mu.Unlock()
		return
	}
	if !b.lastAgentRefresh.IsZero() && time.Since(b.lastAgentRefresh) < b.cfg.AgentFreshnessWindow {
		b.mu.Unlock()
		return
	}
	if !b.isStaleLocked(b.cfg.AgentFreshnessWindow) {
		b.mu.Unlock()
		return
	}
	b.lastAgentRefresh = time.Now().UTC()
	b.mu.Unlock()

	b.enqueueRefresh(reason, 750*time.Millisecond)
}

func (b *Bus) RefreshMissingKey(ctx context.Context, reason string) error {
	for {
		b.mu.Lock()
		if b.stopped {
			b.mu.Unlock()
			return nil
		}
		if b.syncing {
			done := b.syncDone
			b.mu.Unlock()
			if err := waitForSync(ctx, done); err != nil {
				return err
			}
			continue
		}
		b.beginSyncLocked()
		b.mu.Unlock()

		err := b.executePull(ctx, reason)
		b.finishPull(err)
		return err
	}
}

func (b *Bus) LifecycleRefresh(reason string) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	dirty := b.state.Dirty
	b.mu.Unlock()

	if dirty {
		b.enqueuePush("lifecycle:" + reason)
		return
	}
	b.enqueueRefresh(reason, 5*time.Second)
}

func (b *Bus) AuthLinked(ctx context.Context, userID, serverURL string) error {
	for {
		b.mu.Lock()
		if b.stopped {
			b.mu.Unlock()
			return nil
		}
		if b.syncing {
			done := b.syncDone
			b.mu.Unlock()
			if err := waitForSync(ctx, done); err != nil {
				return err
			}
			continue
		}
		b.beginSyncLocked()
		b.mu.Unlock()
		break
	}

	linker, ok := b.engine.(linkRuntime)
	if !ok {
		b.mu.Lock()
		b.state.LinkedUserID = userID
		b.state.ServerURL = serverURL
		b.persistLocked()
		b.mu.Unlock()
		b.finishPull(nil)
		return nil
	}
	state, mutationVersion := b.engineStateSnapshot()
	err := linker.ReconcileOnLink(ctx, &state, userID, serverURL)
	b.applyEngineState(state, mutationVersion)
	b.finishPull(err)
	return err
}

func (b *Bus) AuthUnlinked(_ context.Context) error {
	b.Stop()

	b.mu.Lock()
	defer b.mu.Unlock()
	deviceID := b.state.DeviceID
	cleared := DefaultSyncState(deviceID)
	*b.state = cleared
	b.retryIndex = 0
	b.lastAgentRefresh = time.Time{}
	b.persistLocked()
	return nil
}

func (b *Bus) ForceSync(ctx context.Context, reason string) error {
	for {
		b.mu.Lock()
		if b.stopped {
			b.mu.Unlock()
			return nil
		}
		if b.syncing {
			done := b.syncDone
			if b.state.Dirty {
				b.queuedPush = true
			} else {
				b.queuedRefresh = true
			}
			b.mu.Unlock()
			if err := waitForSync(ctx, done); err != nil {
				return err
			}
			continue
		}

		dirty := b.state.Dirty
		b.beginSyncLocked()
		b.mu.Unlock()

		if dirty {
			err := b.executePush(ctx, reason)
			b.finishPush(err)
			return err
		}

		err := b.executePull(ctx, reason)
		b.finishPull(err)
		return err
	}
}

func (b *Bus) MarkDirty(reason string) {
	b.LocalMutation(reason)
}

func (b *Bus) RequestPull(reason string) {
	b.LifecycleRefresh(reason)
}

func (b *Bus) SnapshotState() SyncState {
	b.mu.Lock()
	defer b.mu.Unlock()

	snapshot := cloneSyncState(*b.state)
	snapshot.Syncing = b.syncing
	return snapshot
}

func (b *Bus) CheckDirtyFlag() {
	if b.cfg.DirtyFlagPath == "" {
		return
	}
	if _, err := os.Stat(b.cfg.DirtyFlagPath); err == nil {
		b.mu.Lock()
		b.mutationVersion++
		b.state.MarkDirty("", time.Time{})
		b.persistLocked()
		_ = os.Remove(b.cfg.DirtyFlagPath)
		b.mu.Unlock()
	}
}

func (b *Bus) enqueuePush(reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}
	if b.syncing {
		b.queuedPush = true
		return
	}
	b.beginSyncLocked()
	go func() {
		err := b.executePush(context.Background(), reason)
		b.finishPush(err)
	}()
}

func (b *Bus) enqueueRefresh(reason string, timeout time.Duration) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	if b.state.Dirty {
		b.mu.Unlock()
		return
	}
	if b.syncing {
		b.queuedRefresh = true
		b.mu.Unlock()
		return
	}
	b.beginSyncLocked()
	b.mu.Unlock()

	go func() {
		ctx := context.Background()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		err := b.executeRefresh(ctx, reason)
		b.finishPull(err)
	}()
}

func (b *Bus) executePush(ctx context.Context, reason string) error {
	ctx, cancel := b.withStopContext(ctx)
	defer cancel()
	state, mutationVersion := b.engineStateSnapshot()
	err := b.engine.PushCurrent(ctx, &state)
	if err == nil {
		b.applyEngineState(state, mutationVersion)
		return nil
	}
	if errors.Is(err, ErrVersionConflict) {
		if merger, ok := b.engine.(mergeRetryRuntime); ok {
			if mergeErr := merger.MergeAndRetry(ctx, &state); mergeErr == nil {
				b.applyEngineState(state, mutationVersion)
				return nil
			} else {
				err = mergeErr
			}
		}
	}
	b.applyEngineState(state, mutationVersion)
	if b.logger != nil {
		b.logger.Debug("push failed", "reason", reason, "error", err)
	}
	return err
}

func (b *Bus) executePull(ctx context.Context, reason string) error {
	ctx, cancel := b.withStopContext(ctx)
	defer cancel()
	state, mutationVersion := b.engineStateSnapshot()
	_, _, err := b.engine.PullLatest(ctx, &state)
	b.applyEngineState(state, mutationVersion)
	if err != nil && b.logger != nil {
		b.logger.Debug("pull failed", "reason", reason, "error", err)
	}
	return err
}

func (b *Bus) executeRefresh(ctx context.Context, reason string) error {
	ctx, cancel := b.withStopContext(ctx)
	defer cancel()
	checker, ok := b.engine.(remoteStatusRuntime)
	if !ok {
		return b.executePull(ctx, reason)
	}

	shouldPull, err := b.remoteNeedsPull(ctx, checker, reason)
	if err != nil {
		return err
	}
	if !shouldPull {
		return nil
	}
	return b.executePull(ctx, reason)
}

func (b *Bus) remoteNeedsPull(ctx context.Context, checker remoteStatusRuntime, reason string) (bool, error) {
	state, _ := b.engineStateSnapshot()
	status, err := checker.RemoteStatus(ctx, &state)
	if errors.Is(err, ErrStatusUnsupported) {
		return true, nil
	}
	if err != nil {
		if b.logger != nil {
			b.logger.Debug("status check failed", "reason", reason, "error", err)
		}
		return false, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state.Dirty || b.stopped {
		return false, nil
	}

	b.state.LastRemoteCheckAt = time.Now().UTC()
	b.state.LastError = ""
	b.state.NextRetryAt = time.Time{}
	shouldPull := status.HasVault && status.Version != b.state.LastKnownServerVersion
	b.persistLocked()
	return shouldPull, nil
}

func (b *Bus) finishPush(err error) {
	b.mu.Lock()
	if err != nil {
		delay := b.nextRetryDelayLocked()
		b.state.MarkDirty(err.Error(), time.Now().UTC().Add(delay))
		b.persistLocked()
		b.scheduleRetryLocked(delay)
	} else {
		b.retryIndex = 0
		if b.retryTimer != nil {
			b.retryTimer.Stop()
			b.retryTimer = nil
		}
		b.persistLocked()
	}

	done := b.syncDone
	b.syncDone = nil
	b.syncing = false
	queuedPush := b.queuedPush
	queuedRefresh := b.queuedRefresh
	if b.stopped {
		queuedPush = false
		queuedRefresh = false
	}
	b.queuedPush = false
	b.queuedRefresh = false
	b.mu.Unlock()

	close(done)
	b.active.Done()

	if err == nil {
		if queuedPush {
			b.enqueuePush("queued_push")
			return
		}
		if queuedRefresh {
			b.enqueueRefresh("queued_refresh", 5*time.Second)
		}
		return
	}

	if queuedPush {
		b.enqueuePush("queued_push_after_error")
	}
}

func (b *Bus) finishPull(err error) {
	b.mu.Lock()
	done := b.syncDone
	b.syncDone = nil
	b.syncing = false
	queuedPush := b.queuedPush
	queuedRefresh := b.queuedRefresh
	if b.stopped {
		queuedPush = false
		queuedRefresh = false
	}
	b.queuedPush = false
	b.queuedRefresh = false
	b.mu.Unlock()

	close(done)
	b.active.Done()

	if queuedPush {
		b.enqueuePush("queued_push")
		return
	}
	_ = queuedRefresh
}

func (b *Bus) beginSyncLocked() {
	b.active.Add(1)
	b.syncing = true
	b.syncDone = make(chan struct{})
}

func (b *Bus) engineStateSnapshot() (SyncState, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return cloneSyncState(*b.state), b.mutationVersion
}

func (b *Bus) applyEngineState(next SyncState, mutationVersion uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	mutatedDuringSync := mutationVersion != b.mutationVersion
	*b.state = cloneSyncState(next)
	if mutatedDuringSync {
		b.state.MarkDirty("", time.Time{})
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		b.queuedPush = true
	}
	b.persistLocked()
}

func cloneSyncState(state SyncState) SyncState {
	state.LastSyncedBaseBlob = append([]byte(nil), state.LastSyncedBaseBlob...)
	return state
}

func (b *Bus) Stop() {
	b.stopOnce.Do(func() {
		b.mu.Lock()
		b.stopped = true
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		if b.retryTimer != nil {
			b.retryTimer.Stop()
			b.retryTimer = nil
		}
		b.queuedPush = false
		b.queuedRefresh = false
		b.mu.Unlock()
		b.cancelStop()
		close(b.stopCh)
		b.active.Wait()
		b.mu.Lock()
		b.persistLocked()
		b.mu.Unlock()
	})
}

func (b *Bus) withStopContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(b.stopCtx, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

func (b *Bus) nextRetryDelayLocked() time.Duration {
	if len(b.cfg.RetryBackoff) == 0 {
		return time.Second
	}
	index := b.retryIndex
	if index >= len(b.cfg.RetryBackoff) {
		index = len(b.cfg.RetryBackoff) - 1
	}
	delay := b.cfg.RetryBackoff[index]
	if b.retryIndex < len(b.cfg.RetryBackoff)-1 {
		b.retryIndex++
	}
	return delay
}

func (b *Bus) scheduleRetryLocked(delay time.Duration) {
	if b.stopped {
		return
	}
	if b.retryTimer != nil {
		b.retryTimer.Stop()
	}
	b.retryTimer = time.AfterFunc(delay, func() {
		b.enqueuePush("retry")
	})
}

func (b *Bus) isStaleLocked(window time.Duration) bool {
	lastFresh := b.state.LastSuccessfulPullAt
	if b.state.LastRemoteCheckAt.After(lastFresh) {
		lastFresh = b.state.LastRemoteCheckAt
	}
	if lastFresh.IsZero() {
		return true
	}
	return time.Since(lastFresh) > window
}

func (b *Bus) startBackgroundRefresh() {
	if b.cfg.RemoteCheckInterval <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(b.cfg.RemoteCheckInterval)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				b.mu.Lock()
				stale := !b.stopped && !b.state.Dirty && b.isStaleLocked(b.cfg.RemoteCheckInterval)
				b.mu.Unlock()
				if stale {
					b.enqueueRefresh("background_status", b.cfg.RemoteCheckTimeout)
				}
				timer.Reset(b.cfg.RemoteCheckInterval)
			case <-b.stopCh:
				return
			}
		}
	}()
}

func (b *Bus) persistLocked() {
	if b.cfg.StateStore != nil {
		if err := b.cfg.StateStore.Save(b.state); err != nil && b.logger != nil {
			b.logger.Warn("persisting sync state failed", "error", err)
		}
	}

	if b.cfg.DirtyFlagPath == "" {
		return
	}

	if b.state.Dirty {
		if err := os.MkdirAll(filepath.Dir(b.cfg.DirtyFlagPath), 0o700); err == nil {
			_ = os.WriteFile(b.cfg.DirtyFlagPath, []byte("1"), 0o600)
		}
		return
	}

	_ = os.Remove(b.cfg.DirtyFlagPath)
}

func waitForSync(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
