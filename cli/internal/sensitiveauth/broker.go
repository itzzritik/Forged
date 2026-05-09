package sensitiveauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	nativeMu  sync.RWMutex
	native    CapabilityState
	systemMu  sync.Mutex
	systemRun *systemAuthCall
	cooldown  systemAuthCooldown
}

type systemAuthCall struct {
	done   chan struct{}
	result systemAuthResult
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

type SessionController interface {
	HasActiveSession() bool
	HydrateFromEnrollment() error
	HydrateFromPassword(password []byte) error
	ClearActiveSession(reason string)
}

func NewBroker(paths config.Paths, helperPath string, logger *slog.Logger, session SessionController) *Broker {
	b := &Broker{
		paths:    paths,
		logger:   logger,
		password: NewPasswordVerifier(paths, logger),
		leases:   newLeaseState(),
		session:  session,
		native:   CapabilityUnavailableByEnv,
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

func (b *Broker) Close() {
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
	if action == ActionExport {
		return AuthorizeResult{
			PasswordRequired: true,
			Prompt:           action.PasswordPrompt(),
		}, nil
	}

	now := time.Now()
	if !force && b.hasActiveSession(now) {
		return b.allow(action, now), nil
	}

	if b.helper != nil {
		capability, err := b.authorizeSystem(ctx, action)
		switch {
		case err == nil:
			b.setNativeCapability(capability)
			if err := b.ensureSessionFromEnrollment(); err != nil {
				return b.handleMissingDeviceUnlock(action, "System Auth succeeded, but this device needs your master password to finish unlocking Forged.", err)
			}
			return b.grant(action, time.Now()), nil
		case errors.Is(err, ErrNativeUnavailable):
			b.setNativeCapability(capability)
			return b.authorizeWithoutSystemAuth(action, capability)
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

	return b.authorizeWithoutSystemAuth(action, b.nativeCapability())
}

func (b *Broker) AuthorizeWithPassword(action Action, password []byte) (AuthorizeResult, error) {
	if err := b.password.Verify(password); err != nil {
		return AuthorizeResult{}, fmt.Errorf("Authentication failed")
	}
	if b.session != nil && !b.session.HasActiveSession() {
		if err := b.session.HydrateFromPassword(password); err != nil {
			return AuthorizeResult{}, fmt.Errorf("Unlocking vault session: %w", err)
		}
	}
	if action == ActionExport {
		return b.allowExport(time.Now()), nil
	}
	return b.grant(action, time.Now()), nil
}

func (b *Broker) IsUnlocked() bool {
	return b.hasActiveSession(time.Now())
}

func (b *Broker) CanViewFull() bool {
	return b.IsUnlocked()
}

func (b *Broker) ConsumeExportToken(token string) bool {
	return b.leases.ConsumeExportToken(token, time.Now())
}

func (b *Broker) Invalidate(reason string) {
	b.Lock(reason)
}

func (b *Broker) Lock(reason string) {
	b.clearSharedSession(reason)
}

func (b *Broker) hasActiveSession(now time.Time) bool {
	if b.leases.IsExpired(now) {
		b.clearSharedSession("session_expired")
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

func (b *Broker) clearSharedSession(reason string) {
	b.leases.Clear()
	if b.session != nil {
		b.session.ClearActiveSession(reason)
	}
	if b.logger != nil {
		b.logger.Info("sensitive auth invalidated", "reason", reason)
	}
}

func (b *Broker) grant(action Action, now time.Time) AuthorizeResult {
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

func (b *Broker) allowExport(now time.Time) AuthorizeResult {
	return AuthorizeResult{
		Authorized:  true,
		ExportToken: b.leases.IssueExportToken(now),
	}
}

func (b *Broker) authorizeSystem(ctx context.Context, action Action) (CapabilityState, error) {
	if b.helper == nil {
		return b.nativeCapability(), ErrNativeUnavailable
	}
	if action == ActionExternal {
		if err := b.externalCooldownErr(time.Now()); err != nil {
			return b.nativeCapability(), err
		}
	}

	b.systemMu.Lock()
	if call := b.systemRun; call != nil {
		b.systemMu.Unlock()
		select {
		case <-call.done:
			return call.result.capability, call.result.err
		case <-ctx.Done():
			return CapabilityBroken, ctx.Err()
		}
	}
	call := &systemAuthCall{done: make(chan struct{})}
	b.systemRun = call
	b.systemMu.Unlock()

	capability, err := b.helper.Authorize(ctx, action)
	call.result = systemAuthResult{capability: capability, err: err}

	b.systemMu.Lock()
	if b.systemRun == call {
		b.systemRun = nil
	}
	close(call.done)
	b.systemMu.Unlock()

	return capability, err
}

func (b *Broker) authorizeWithoutSystemAuth(action Action, capability CapabilityState) (AuthorizeResult, error) {
	if !isHeadlessAuthMode(capability) {
		if action == ActionExternal {
			return AuthorizeResult{}, externalUseBrokenError()
		}
		return passwordRequired(action.PasswordPrompt()), nil
	}

	if err := b.ensureSessionFromEnrollment(); err != nil {
		return b.handleMissingDeviceUnlock(action, "Enter your master password to unlock this device.", err)
	}
	if b.logger != nil {
		b.logger.Info("allowing use without System Auth", "action", action, "capability", capability)
	}
	return b.grant(action, time.Now()), nil
}

func isHeadlessAuthMode(capability CapabilityState) bool {
	if !capability.IsUnavailable() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("FORGED_HEADLESS")), "1") {
		return true
	}
	return runtime.GOOS == "linux"
}

func (b *Broker) ensureSessionFromEnrollment() error {
	if b.session == nil || b.session.HasActiveSession() {
		return nil
	}
	return b.session.HydrateFromEnrollment()
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
