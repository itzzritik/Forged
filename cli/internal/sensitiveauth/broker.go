package sensitiveauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

type Broker struct {
	logger   *slog.Logger
	helper   *HelperClient
	password *PasswordVerifier
	leases   *leaseState
	session  SessionController
}

type SessionController interface {
	HasActiveSession() bool
	HydrateFromEnrollment() error
	HydrateFromPassword(password []byte) error
	ClearActiveSession(reason string)
}

func NewBroker(paths config.Paths, helperPath string, logger *slog.Logger, session SessionController) *Broker {
	b := &Broker{
		logger:   logger,
		password: NewPasswordVerifier(paths, logger),
		leases:   newLeaseState(),
		session:  session,
	}

	if helperPath != "" {
		helper := NewHelperClient(helperPath, logger)
		if err := helper.Start(context.Background(), func() { b.Invalidate("system_lock") }); err != nil {
			if b.logger != nil {
				b.logger.Debug("sensitive auth helper unavailable", "error", err, "path", helperPath)
			}
		} else {
			b.helper = helper
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
	now := time.Now()
	if !force && b.hasActiveSession(now) {
		return b.allow(action, now), nil
	}

	if b.helper != nil {
		err := b.helper.Authorize(ctx, action)
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
			return b.grant(action, time.Now()), nil
		case errors.Is(err, ErrNativeUnavailable):
			// Fall through to terminal password prompt.
		case errors.Is(err, ErrAuthenticationCanceled):
			return AuthorizeResult{}, fmt.Errorf("authentication canceled")
		default:
			return AuthorizeResult{}, fmt.Errorf("authentication failed")
		}
	}

	return AuthorizeResult{
		PasswordRequired: true,
		Prompt:           action.PasswordPrompt(),
	}, nil
}

func (b *Broker) AuthorizeWithPassword(action Action, password []byte) (AuthorizeResult, error) {
	if err := b.password.Verify(password); err != nil {
		return AuthorizeResult{}, fmt.Errorf("authentication failed")
	}
	if b.session != nil && !b.session.HasActiveSession() {
		if err := b.session.HydrateFromPassword(password); err != nil {
			return AuthorizeResult{}, fmt.Errorf("unlocking vault session: %w", err)
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
