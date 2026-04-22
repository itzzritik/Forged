//go:build !darwin && !windows

package sensitiveauth

import "context"

type stubSecureStore struct{}

func newPlatformSecureStore() SecureStore {
	return &stubSecureStore{}
}

func (s *stubSecureStore) Capability(context.Context) CapabilityState {
	return CapabilityUnavailableByPlatform
}

func (s *stubSecureStore) SaveDeviceKey(context.Context, string, []byte) error {
	return ErrSecureStoreUnavailable
}

func (s *stubSecureStore) LoadDeviceKey(context.Context, string) ([]byte, error) {
	return nil, ErrSecureStoreUnavailable
}

func (s *stubSecureStore) DeleteDeviceKey(context.Context, string) error {
	return ErrSecureStoreUnavailable
}
