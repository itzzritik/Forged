package sensitiveauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

type Broker struct {
	paths    config.Paths
	logger   *slog.Logger
	helper   *HelperClient
	password *PasswordVerifier
	leases   *leaseState
	session  SessionController
	nativeMu sync.RWMutex
	native   CapabilityState
}

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
		native:   CapabilityBroken,
	}

	if helperPath != "" {
		helper := NewHelperClient(helperPath, logger)
		if err := helper.Start(context.Background(), func() { b.Invalidate("system_lock") }); err != nil {
			if b.logger != nil {
				b.logger.Debug("sensitive auth helper unavailable", "error", err, "path", helperPath)
			}
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
		capability, err := b.helper.Authorize(ctx, action)
		switch {
		case err == nil:
			if b.session != nil && !b.session.HasActiveSession() {
				if err := b.session.HydrateFromEnrollment(); err != nil {
					if b.logger != nil {
						b.logger.Warn("native auth succeeded but local unlock hydration failed", "error", err)
					}
					return AuthorizeResult{
						PasswordRequired: true,
						Prompt:           "Authentication worked, but local unlock trust needs your master password.",
					}, nil
				}
			}
			b.setNativeCapability(capability)
			return b.grant(action, time.Now()), nil
		case errors.Is(err, ErrNativeUnavailable):
			b.setNativeCapability(capability)
			if action == ActionExternal {
				return b.authorizeExternalUnavailable(action, capability)
			}
		case errors.Is(err, ErrNativeBroken):
			b.setNativeCapability(CapabilityBroken)
			if action == ActionExternal {
				return AuthorizeResult{}, externalUseBrokenError()
			}
		case errors.Is(err, ErrAuthenticationCanceled):
			if action == ActionExternal {
				return AuthorizeResult{}, externalUseCanceledError()
			}
			return AuthorizeResult{}, fmt.Errorf("Authentication canceled")
		default:
			if action == ActionExternal {
				return AuthorizeResult{}, externalUseFailedError()
			}
			return AuthorizeResult{}, fmt.Errorf("Authentication failed")
		}
	}

	if action == ActionExternal {
		capability := b.nativeCapability()
		if capability.IsUnavailable() {
			return b.authorizeExternalUnavailable(action, capability)
		}
		return AuthorizeResult{}, externalUseBrokenError()
	}

	return AuthorizeResult{
		PasswordRequired: true,
		Prompt:           action.PasswordPrompt(),
	}, nil
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
	return b.grant(action, time.Now()), nil
}

func (b *Broker) IsUnlocked() bool {
	return b.hasActiveSession(time.Now())
}

func (b *Broker) CanViewFull() bool {
	return b.IsUnlocked()
}

func (b *Broker) ConsumeExportToken(token string) bool {
	now := time.Now()
	if !b.hasActiveSession(now) {
		return false
	}
	return b.leases.ConsumeExportToken(token, now)
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

func (b *Broker) authorizeExternalUnavailable(action Action, capability CapabilityState) (AuthorizeResult, error) {
	if action != ActionExternal {
		return AuthorizeResult{}, fmt.Errorf("External unavailable policy only applies to external use")
	}

	if externalUsePolicy(b.paths) != config.ExternalUsePolicyAllow {
		return AuthorizeResult{}, externalUseDeniedError()
	}

	if b.session != nil && !b.session.HasActiveSession() {
		if err := b.session.HydrateFromEnrollment(); err != nil {
			if errors.Is(err, ErrLocalUnlockTrustUnavailable) {
				return AuthorizeResult{}, externalUseNoLocalTrustError()
			}
			return AuthorizeResult{}, externalUseHydrationError()
		}
	}

	if b.logger != nil {
		b.logger.Warn("allowing external use without system authentication", "capability", capability)
	}
	return b.grant(action, time.Now()), nil
}

func externalUseBrokenError() error {
	return fmt.Errorf("System authentication is broken; repair Forged before using SSH auth or commit signing")
}

func externalUseCanceledError() error {
	return fmt.Errorf("System authentication was canceled")
}

func externalUseFailedError() error {
	return fmt.Errorf("System authentication failed")
}

func externalUseDeniedError() error {
	return fmt.Errorf("System authentication is unavailable on this machine, and external use is denied")
}

func externalUseNoLocalTrustError() error {
	return fmt.Errorf("System authentication is unavailable on this machine, and local unlock trust is not available")
}

func externalUseHydrationError() error {
	return fmt.Errorf("System authentication is unavailable on this machine, and Forged could not unlock local trust")
}

func externalUsePolicy(paths config.Paths) string {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return config.ExternalUsePolicyDeny
	}
	return cfg.Security.ExternalUsePolicy
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
