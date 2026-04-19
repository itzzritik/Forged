package sensitiveauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type Broker struct {
	logger   *slog.Logger
	helper   *HelperClient
	password *PasswordVerifier
	leases   *leaseState
}

func NewBroker(vaultPath, helperPath string, logger *slog.Logger) *Broker {
	b := &Broker{
		logger:   logger,
		password: NewPasswordVerifier(vaultPath),
		leases:   newLeaseState(),
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
	if b.IsUnlocked() {
		return b.grant(action), nil
	}

	if b.helper != nil {
		err := b.helper.Authorize(ctx, action)
		switch {
		case err == nil:
			return b.grant(action), nil
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
	return b.grant(action), nil
}

func (b *Broker) IsUnlocked() bool {
	return b.leases.IsUnlocked()
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
	b.leases.Clear()
	if b.logger != nil {
		b.logger.Info("sensitive auth invalidated", "reason", reason)
	}
}

func (b *Broker) grant(action Action) AuthorizeResult {
	now := time.Now()
	b.leases.GrantView(now)
	result := AuthorizeResult{Authorized: true}
	if action == ActionExport {
		result.ExportToken = b.leases.IssueExportToken(now)
	}
	return result
}
