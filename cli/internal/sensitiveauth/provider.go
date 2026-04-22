package sensitiveauth

import (
	"context"
	"errors"
)

var (
	ErrNativeUnavailable      = errors.New("Native authentication unavailable")
	ErrNativeBroken           = errors.New("Native authentication broken")
	ErrAuthenticationCanceled = errors.New("Authentication canceled")
	ErrAuthenticationFailed   = errors.New("Authentication failed")
)

type NativeProvider interface {
	Name() string
	Authorize(ctx context.Context, action Action) error
}

type LockWatcher interface {
	Start(onLock func()) error
	Stop() error
}

type noopLockWatcher struct{}

func (noopLockWatcher) Start(func()) error { return nil }
func (noopLockWatcher) Stop() error        { return nil }

type unavailableProvider struct{}

func (unavailableProvider) Name() string { return "native" }

func (unavailableProvider) Authorize(context.Context, Action) error {
	return ErrNativeUnavailable
}
