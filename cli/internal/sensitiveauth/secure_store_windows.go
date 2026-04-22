//go:build windows

package sensitiveauth

import "context"

type windowsSecureStore struct{}

func newPlatformSecureStore() SecureStore {
	return &windowsSecureStore{}
}

func (s *windowsSecureStore) Capability(context.Context) CapabilityState {
	return CapabilityUnavailableByPlatform
}

func (s *windowsSecureStore) SaveDeviceKey(context.Context, string, []byte) error {
	return ErrSecureStoreUnavailable
}

func (s *windowsSecureStore) LoadDeviceKey(context.Context, string) ([]byte, error) {
	return nil, ErrSecureStoreUnavailable
}

func (s *windowsSecureStore) DeleteDeviceKey(context.Context, string) error {
	return ErrSecureStoreUnavailable
}
