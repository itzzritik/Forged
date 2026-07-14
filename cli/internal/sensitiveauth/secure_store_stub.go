//go:build !darwin && !windows

package sensitiveauth

import (
	"context"

	"github.com/itzzritik/forged/cli/internal/config"
)

type stubSecureStore struct{}

func newPlatformSecureStore(config.Paths) SecureStore {
	return &stubSecureStore{}
}

func (s *stubSecureStore) Capability(context.Context) CapabilityState {
	return CapabilityUnavailableByPlatform
}

func (s *stubSecureStore) SaveDeviceKey(context.Context, string, string, []byte) error {
	return ErrSecureStoreUnavailable
}

func (s *stubSecureStore) LoadDeviceKey(context.Context, string, string) ([]byte, error) {
	return nil, ErrSecureStoreUnavailable
}

func (s *stubSecureStore) DeleteDeviceKey(context.Context, string, string) error {
	return ErrSecureStoreUnavailable
}
