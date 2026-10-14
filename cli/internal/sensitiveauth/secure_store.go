package sensitiveauth

import (
	"context"
	"errors"

	"github.com/itzzritik/forged/cli/internal/config"
)

var (
	ErrSecureStoreUnavailable = errors.New("Secure storage unavailable")
	ErrSecureStoreBroken      = errors.New("Secure storage broken")
	ErrSecureStoreNotFound    = errors.New("Secure storage item not found")
)

type SecureStore interface {
	Capability(ctx context.Context) CapabilityState
	SaveDeviceKey(ctx context.Context, installID, slot string, key []byte) error
	LoadDeviceKey(ctx context.Context, installID, slot string) ([]byte, error)
	DeleteDeviceKey(ctx context.Context, installID, slot string) error
}

func NewSecureStore(paths config.Paths) SecureStore {
	return newPlatformSecureStore(paths)
}
