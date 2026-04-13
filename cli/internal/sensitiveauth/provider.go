package sensitiveauth

import (
	"context"
	"errors"
)

var (
	ErrNativeUnavailable      = errors.New("native authentication unavailable")
	ErrAuthenticationCanceled = errors.New("authentication canceled")
	ErrAuthenticationFailed   = errors.New("authentication failed")
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
