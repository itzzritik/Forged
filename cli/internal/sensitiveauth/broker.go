package sensitiveauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

type Broker struct {
	paths     config.Paths
	logger    *slog.Logger
	helper    *HelperClient
	password  *PasswordVerifier
	leases    *leaseState
	session   SessionController
	sessionMu sync.Mutex
	// authGeneration is guarded by sessionMu. It serializes a lock event with
	// the final session grant, so a pre-lock prompt cannot restore a session.
	authGeneration uint64
	nativeMu       sync.RWMutex
	native         CapabilityState
	systemMu       sync.Mutex
	systemRun      *systemAuthCall
	cooldown       systemAuthCooldown
	pwMu           sync.Mutex
	pwRun          *passwordUnlockCall
	pwCooldown     time.Time
	lifecycleMu    sync.Mutex
	stopping       bool
	stopOnce       sync.Once
	background     sync.WaitGroup
	stopCtx        context.Context
	stopCancel     context.CancelFunc
}

type systemAuthCall struct {
	done       chan struct{}
	cancel     context.CancelFunc
	generation uint64
	waiters    int
	complete   bool
	abandoned  bool
	result     systemAuthResult
}

type passwordUnlockCall struct {
	cancel context.CancelFunc
}

type systemAuthResult struct {
	capability CapabilityState
	err        error
}

type systemAuthCooldown struct {
	until time.Time
	err   error
}

const externalPromptCooldown = 10 * time.Second

const externalPasswordReason = "Enter your Forged master password to keep SSH working."

type SessionController interface {
	HasActiveSession() bool
	HydrateFromEnrollment() error
	HydrateFromPassword(password []byte) error
	ClearActiveSession(reason string)
}

func NewBroker(paths config.Paths, helperPath string, logger *slog.Logger, session SessionController) *Broker {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	b := &Broker{
		paths:      paths,
		logger:     logger,
		password:   NewPasswordVerifier(paths, logger),
		leases:     newLeaseState(),
		session:    session,
		native:     CapabilityUnavailableByEnv,
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
	}

	if helperPath != "" {
		helper := NewHelperClient(helperPath, logger)
		if err := helper.Start(context.Background(), func() { b.Invalidate("system_lock") }); err != nil {
			if b.logger != nil {
				b.logger.Debug("sensitive auth helper unavailable", "error", err, "path", helperPath)
			}
			b.native = CapabilityUnavailableByEnv
		} else {
			b.helper = helper
			b.native = CapabilityAvailable
		}
	}

	return b
}

func (b *Broker) BeginStop() {
	b.stopOnce.Do(func() {
		b.lifecycleMu.Lock()
		b.stopping = true
		b.lifecycleMu.Unlock()
		b.stopCancel()
	})
}

func (b *Broker) Wait() {
	b.background.Wait()
	b.systemMu.Lock()
	call := b.systemRun
	b.systemMu.Unlock()
	if call != nil {
		<-call.done
	}
}

func (b *Broker) Close() {
	b.BeginStop()
	b.Wait()
	if b.helper != nil {
		_ = b.helper.Close()
	}
	b.Invalidate("shutdown")
}

func (b *Broker) Authorize(ctx context.Context, action Action) (AuthorizeResult, error) {
	return b.authorize(ctx, action, false)
}

func (b *Broker) AuthorizeForced(ctx context.Context, action Action) (AuthorizeResult, error) {
	return b.authorize(ctx, action, true)
}

func (b *Broker) authorize(ctx context.Context, action Action, force bool) (AuthorizeResult, error) {
	if action == ActionExport || action == ActionPrivateKey {
		return AuthorizeResult{
			PasswordRequired: true,
			Prompt:           action.PasswordPrompt(),
		}, nil
	}

	generation := b.authorizationGeneration()
	now := time.Now()
	result, activeSession, err := b.allowActiveSession(action, now, generation)
	if errors.Is(err, ErrAuthenticationCanceled) {
		return b.authorizationInterrupted(action)
	}
	if !force && activeSession {
		return result, nil
	}
	if headlessModePreemptsSystemAuth(b.paths) {
		return b.authorizeWithoutSystemAuth(action, CapabilityUnavailableByPlatform, generation)
	}

	if b.helper != nil {
		// External use can't fall back to the TUI's master-password page. Once the
		// device-unlock window has lapsed (or was never enrolled), a Touch ID prompt
		// can't restart it. ssh won't wait for a human, so we fire the master-password
		// popup in the background to unlock the shared session and fail this request;
		// the next connection then succeeds.
		// ponytail: darwin-only gate — the collect-password popup is macOS-only for
		// now. Extend when Windows CredUI / Linux zenity land.
		if action == ActionExternal && !LocalEnrollmentUsable(b.paths) {
			if runtime.GOOS == "darwin" {
				b.promptPasswordUnlock(generation)
				return AuthorizeResult{}, externalUseLockedError()
			}
			return AuthorizeResult{}, externalUseNoDeviceUnlockError()
		}
		capability, err := b.authorizeSystem(ctx, action, generation)
		if !b.authorizationCurrent(generation) {
			return b.authorizationInterrupted(action)
		}
		switch {
		case err == nil:
			b.setNativeCapability(capability)
			result, err := b.grantWithEnrollment(action, time.Now(), generation)
			if errors.Is(err, ErrAuthenticationCanceled) {
				return b.authorizationInterrupted(action)
			}
			if err != nil {
				return b.handleMissingDeviceUnlock(action, "System Auth succeeded, but this device needs your master password to finish unlocking Forged.", err)
			}
			return result, nil
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return AuthorizeResult{}, err
		case errors.Is(err, ErrNativeUnavailable):
			b.setNativeCapability(capability)
			return b.authorizeWithoutSystemAuth(action, capability, generation)
		case errors.Is(err, ErrNativeBroken):
			b.setNativeCapability(CapabilityBroken)
			if action == ActionExternal {
				return AuthorizeResult{}, externalUseBrokenError()
			}
			return passwordRequired("System Auth is not working. Enter your master password to unlock Forged."), nil
		case errors.Is(err, ErrAuthenticationCanceled):
			if action == ActionExternal {
				b.recordExternalCooldown(err)
				return AuthorizeResult{}, externalUseCanceledError()
			}
			return passwordRequired("System Auth was canceled. Try System Auth again, or enter your master password."), nil
		default:
			if action == ActionExternal {
				b.recordExternalCooldown(err)
				return AuthorizeResult{}, externalUseFailedError()
			}
			return passwordRequired("System Auth failed. Enter your master password to continue."), nil
		}
	}

	return b.authorizeWithoutSystemAuth(action, b.nativeCapability(), generation)
}

func (b *Broker) AuthorizeWithPassword(action Action, password []byte) (AuthorizeResult, error) {
	return b.authorizeWithPassword(action, password, b.authorizationGeneration())
}

func (b *Broker) authorizeWithPassword(action Action, password []byte, generation uint64) (AuthorizeResult, error) {
	if !b.authorizationCurrent(generation) {
		return AuthorizeResult{}, ErrAuthenticationCanceled
	}
	if err := b.password.Verify(password); err != nil {
		return AuthorizeResult{}, fmt.Errorf("Authentication failed")
	}

	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if b.authGeneration != generation {
		return AuthorizeResult{}, ErrAuthenticationCanceled
	}
	hydrated := false
	if b.session != nil && !b.session.HasActiveSession() {
		if err := b.session.HydrateFromPassword(password); err != nil {
			return AuthorizeResult{}, fmt.Errorf("Unlocking vault session: %w", err)
		}
		hydrated = true
	}
	if b.authGeneration != generation {
		if hydrated {
			b.clearSharedSessionLocked("authorization_invalidated")
		}
		return AuthorizeResult{}, ErrAuthenticationCanceled
	}
	now := time.Now()
	if action == ActionExport {
		return b.grantLocked(action, now), nil
	}
	result := b.grantLocked(action, now)
	if action == ActionPrivateKey {
		result.PrivateKeyToken = b.leases.IssuePrivateKeyToken(now)
	}
	return result, nil
}

func (b *Broker) IsUnlocked() bool {
	return b.hasActiveSession(time.Now())
}

func (b *Broker) ConsumeExportToken(token string) bool {
	return b.leases.ConsumeExportToken(token, time.Now())
}

func (b *Broker) ConsumePrivateKeyToken(token string) bool {
	return b.leases.ConsumePrivateKeyToken(token, time.Now())
}

func (b *Broker) Invalidate(reason string) {
	b.Lock(reason)
}

func (b *Broker) Lock(reason string) {
	b.sessionMu.Lock()
	b.authGeneration++
	b.clearSharedSessionLocked(reason)
	b.sessionMu.Unlock()
	b.cancelSystemAuthForLock()
	b.cancelPasswordUnlockForLock()
}

func (b *Broker) cancelSystemAuthForLock() {
	b.systemMu.Lock()
	call := b.systemRun
	b.systemMu.Unlock()
	if call != nil {
		call.cancel()
	}
}

func (b *Broker) cancelPasswordUnlockForLock() {
	b.pwMu.Lock()
	call := b.pwRun
	b.pwMu.Unlock()
	if call != nil {
		call.cancel()
	}
}

func (b *Broker) authorizationGeneration() uint64 {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	return b.authGeneration
}

func (b *Broker) authorizationCurrent(generation uint64) bool {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	return b.authGeneration == generation
}

func (b *Broker) hasActiveSession(now time.Time) bool {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	return b.hasActiveSessionLocked(now)
}

func (b *Broker) hasActiveSessionLocked(now time.Time) bool {

	if b.leases.IsExpired(now) {
		b.clearSharedSessionLocked("session_expired")
		return false
	}
	if !b.leases.IsUnlocked(now) {
		return false
	}
	if b.session != nil && !b.session.HasActiveSession() {
		b.leases.Clear()
		return false
	}
	return true
}

func (b *Broker) allowActiveSession(action Action, now time.Time, generation uint64) (AuthorizeResult, bool, error) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()

	if b.authGeneration != generation {
		return AuthorizeResult{}, false, ErrAuthenticationCanceled
	}
	if !b.hasActiveSessionLocked(now) {
		return AuthorizeResult{}, false, nil
	}
	return b.allow(action, now), true, nil
}

func (b *Broker) clearSharedSessionLocked(reason string) {
	b.leases.Clear()
	if b.session != nil {
		b.session.ClearActiveSession(reason)
	}
	if b.logger != nil {
		b.logger.Info("sensitive auth invalidated", "reason", reason)
	}
}

func (b *Broker) grantLocked(action Action, now time.Time) AuthorizeResult {
	b.leases.GrantView(now)
	result := AuthorizeResult{Authorized: true}
	if action == ActionExport {
		result.ExportToken = b.leases.IssueExportToken(now)
	}
	return result
}

func (b *Broker) allow(action Action, now time.Time) AuthorizeResult {
	result := AuthorizeResult{Authorized: true}
	if action == ActionExport {
		result.ExportToken = b.leases.IssueExportToken(now)
	}
	return result
}

// promptPasswordUnlock opens the master-password popup in the background and
// returns immediately — the triggering SSH request is expected to fail, since
// ssh won't wait for a human. Entering the password unlocks the shared session
// so the next connection succeeds. Single-flight + cooldown so a retry storm
// shows one popup and does not re-nag after a dismissal.
func (b *Broker) promptPasswordUnlock(generation uint64) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if b.authGeneration != generation {
		return
	}

	b.pwMu.Lock()
	if b.pwRun != nil || time.Now().Before(b.pwCooldown) {
		b.pwMu.Unlock()
		return
	}
	b.lifecycleMu.Lock()
	if b.stopping {
		b.lifecycleMu.Unlock()
		b.pwMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(b.stopCtx)
	call := &passwordUnlockCall{cancel: cancel}
	b.pwRun = call
	b.background.Add(1)
	b.lifecycleMu.Unlock()
	b.pwMu.Unlock()

	go func() {
		defer b.background.Done()
		defer cancel()
		setCooldown := b.runPasswordUnlock(ctx, generation)
		b.pwMu.Lock()
		if b.pwRun == call {
			b.pwRun = nil
			if setCooldown {
				b.pwCooldown = time.Now().Add(externalPromptCooldown)
			}
		}
		b.pwMu.Unlock()
	}()
}

func (b *Broker) runPasswordUnlock(ctx context.Context, generation uint64) bool {
	if !b.authorizationCurrent(generation) {
		return false
	}
	password, err := b.helper.CollectPassword(ctx, externalPasswordReason)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || !b.authorizationCurrent(generation) {
			return false
		}
		if b.logger != nil && !errors.Is(err, ErrAuthenticationCanceled) {
			b.logger.Warn("master-password popup unavailable", "error", err)
		}
		return true
	}
	defer zeroSensitiveBytes(password)
	if !b.authorizationCurrent(generation) {
		return false
	}

	// Verifies against the vault, hydrates the shared session, and (via
	// PasswordVerifier) refreshes the device-unlock enrollment — restarting the
	// sliding window and hard cap. Subsequent SSH connections then pass.
	if _, err := b.authorizeWithPassword(ActionExternal, password, generation); err != nil &&
		b.logger != nil && !errors.Is(err, ErrAuthenticationCanceled) {
		b.logger.Warn("master-password unlock failed", "error", err)
	}
	return true
}

func (b *Broker) authorizeSystem(ctx context.Context, action Action, generation uint64) (CapabilityState, error) {
	if err := ctx.Err(); err != nil {
		return CapabilityBroken, err
	}
	if !b.authorizationCurrent(generation) {
		return CapabilityBroken, ErrAuthenticationCanceled
	}
	if b.helper == nil {
		return b.nativeCapability(), ErrNativeUnavailable
	}
	if action == ActionExternal {
		if err := b.externalCooldownErr(time.Now()); err != nil {
			return b.nativeCapability(), err
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return CapabilityBroken, err
		}
		b.sessionMu.Lock()
		if b.authGeneration != generation {
			b.sessionMu.Unlock()
			return CapabilityBroken, ErrAuthenticationCanceled
		}
		b.lifecycleMu.Lock()
		if b.stopping {
			b.lifecycleMu.Unlock()
			b.sessionMu.Unlock()
			return CapabilityBroken, context.Canceled
		}
		b.systemMu.Lock()
		call := b.systemRun
		if call != nil && (call.abandoned || call.generation != generation) {
			done := call.done
			b.systemMu.Unlock()
			b.lifecycleMu.Unlock()
			b.sessionMu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return CapabilityBroken, ctx.Err()
			}
		}
		if call == nil {
			callCtx, cancel := context.WithCancel(b.stopCtx)
			call = &systemAuthCall{done: make(chan struct{}), cancel: cancel, generation: generation}
			b.systemRun = call
			go b.runSystemAuth(call, callCtx, action)
		}
		call.waiters++
		b.systemMu.Unlock()
		b.lifecycleMu.Unlock()
		b.sessionMu.Unlock()

		select {
		case <-call.done:
			b.releaseSystemAuthWaiter(call)
			return call.result.capability, call.result.err
		case <-ctx.Done():
			if result, complete := b.cancelSystemAuthWaiter(call); complete {
				return result.capability, result.err
			}
			return CapabilityBroken, ctx.Err()
		}
	}
}

func (b *Broker) runSystemAuth(call *systemAuthCall, ctx context.Context, action Action) {
	capability, err := b.helper.Authorize(ctx, action)
	b.systemMu.Lock()
	call.result = systemAuthResult{capability: capability, err: err}
	call.complete = true
	if b.systemRun == call {
		b.systemRun = nil
	}
	close(call.done)
	b.systemMu.Unlock()
	call.cancel()
}

func (b *Broker) releaseSystemAuthWaiter(call *systemAuthCall) {
	b.systemMu.Lock()
	if call.waiters > 0 {
		call.waiters--
	}
	b.systemMu.Unlock()
}

func (b *Broker) cancelSystemAuthWaiter(call *systemAuthCall) (systemAuthResult, bool) {
	b.systemMu.Lock()
	if call.waiters > 0 {
		call.waiters--
	}
	if call.complete {
		result := call.result
		b.systemMu.Unlock()
		return result, true
	}
	cancel := call.waiters == 0 && !call.abandoned
	if cancel {
		call.abandoned = true
	}
	b.systemMu.Unlock()
	if cancel {
		call.cancel()
	}
	return systemAuthResult{}, false
}

func (b *Broker) authorizeWithoutSystemAuth(action Action, capability CapabilityState, generation uint64) (AuthorizeResult, error) {
	if !isHeadlessAuthMode(b.paths, capability) {
		if action == ActionExternal {
			return AuthorizeResult{}, externalUseBrokenError()
		}
		return passwordRequired(action.PasswordPrompt()), nil
	}

	result, err := b.grantWithEnrollment(action, time.Now(), generation)
	if errors.Is(err, ErrAuthenticationCanceled) {
		return b.authorizationInterrupted(action)
	}
	if err != nil {
		return b.handleMissingDeviceUnlock(action, "Enter your master password to unlock this device.", err)
	}
	if b.logger != nil {
		b.logger.Info("allowing use without System Auth", "action", action, "capability", capability)
	}
	return result, nil
}

func isHeadlessAuthMode(paths config.Paths, capability CapabilityState) bool {
	if !capability.IsUnavailable() {
		return false
	}
	return HeadlessModeEnabled(paths)
}

func (b *Broker) grantWithEnrollment(action Action, now time.Time, generation uint64) (AuthorizeResult, error) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()

	if b.authGeneration != generation {
		return AuthorizeResult{}, ErrAuthenticationCanceled
	}
	if b.session == nil || b.session.HasActiveSession() {
		if b.authGeneration != generation {
			return AuthorizeResult{}, ErrAuthenticationCanceled
		}
		return b.grantLocked(action, now), nil
	}
	if err := b.session.HydrateFromEnrollment(); err != nil {
		return AuthorizeResult{}, err
	}
	if b.authGeneration != generation {
		b.clearSharedSessionLocked("authorization_invalidated")
		return AuthorizeResult{}, ErrAuthenticationCanceled
	}
	return b.grantLocked(action, now), nil
}

func (b *Broker) authorizationInterrupted(action Action) (AuthorizeResult, error) {
	if action == ActionExternal {
		return AuthorizeResult{}, externalUseLockedError()
	}
	return passwordRequired("Forged was locked while authenticating. Enter your master password to unlock it."), nil
}

func (b *Broker) handleMissingDeviceUnlock(action Action, prompt string, err error) (AuthorizeResult, error) {
	if b.logger != nil {
		b.logger.Warn("device unlock hydration failed", "action", action, "error", err)
	}
	if action == ActionExternal {
		if errors.Is(err, ErrLocalUnlockTrustUnavailable) {
			return AuthorizeResult{}, externalUseNoDeviceUnlockError()
		}
		return AuthorizeResult{}, externalUseHydrationError()
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = action.PasswordPrompt()
	}
	return passwordRequired(prompt), nil
}

func passwordRequired(prompt string) AuthorizeResult {
	return AuthorizeResult{
		PasswordRequired: true,
		Prompt:           strings.TrimSpace(prompt),
	}
}

func (b *Broker) externalCooldownErr(now time.Time) error {
	b.systemMu.Lock()
	defer b.systemMu.Unlock()
	if b.cooldown.err == nil || b.cooldown.until.IsZero() || !now.Before(b.cooldown.until) {
		return nil
	}
	return b.cooldown.err
}

func (b *Broker) recordExternalCooldown(err error) {
	if err == nil {
		return
	}
	b.systemMu.Lock()
	defer b.systemMu.Unlock()
	b.cooldown = systemAuthCooldown{
		until: time.Now().Add(externalPromptCooldown),
		err:   err,
	}
}

func externalUseBrokenError() error {
	return fmt.Errorf("System Auth is not working; repair Forged before using SSH auth or commit signing")
}

func externalUseCanceledError() error {
	return fmt.Errorf("System Auth was canceled")
}

func externalUseFailedError() error {
	return fmt.Errorf("System Auth failed")
}

func externalUseLockedError() error {
	return fmt.Errorf("Forged is locked; enter your master password in the Forged prompt, then retry")
}

func externalUseNoDeviceUnlockError() error {
	return fmt.Errorf("Device unlock is not enrolled; open Forged and enter your master password once before using SSH auth or commit signing")
}

func externalUseHydrationError() error {
	return fmt.Errorf("Forged could not unlock this device for SSH auth or commit signing")
}

func (b *Broker) setNativeCapability(capability CapabilityState) {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	b.native = capability
}

func (b *Broker) nativeCapability() CapabilityState {
	b.nativeMu.RLock()
	defer b.nativeMu.RUnlock()
	return b.native
}
