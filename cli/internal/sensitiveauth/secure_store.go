package sensitiveauth

import (
	"context"
	"errors"
)

var (
	ErrSecureStoreUnavailable = errors.New("Secure storage unavailable")
	ErrSecureStoreBroken      = errors.New("Secure storage broken")
	ErrSecureStoreNotFound    = errors.New("Secure storage item not found")
)

type SecureStore interface {
	Capability(ctx context.Context) CapabilityState
	SaveDeviceKey(ctx context.Context, installID string, key []byte) error
	LoadDeviceKey(ctx context.Context, installID string) ([]byte, error)
	DeleteDeviceKey(ctx context.Context, installID string) error
}

func NewSecureStore() SecureStore {
	return newPlatformSecureStore()
}
