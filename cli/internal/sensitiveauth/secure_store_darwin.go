//go:build darwin

package sensitiveauth

import (
	"context"
	"encoding/base64"
	"os/exec"
	"strings"
)

const darwinSecureStoreService = "com.forged.local-unlock"

type darwinSecureStore struct{}

func newPlatformSecureStore() SecureStore {
	return &darwinSecureStore{}
}

func (s *darwinSecureStore) Capability(context.Context) CapabilityState {
	if _, err := exec.LookPath("security"); err != nil {
		return CapabilityBroken
	}
	return CapabilityAvailable
}

func (s *darwinSecureStore) SaveDeviceKey(ctx context.Context, installID string, key []byte) error {
	if !s.Capability(ctx).IsAvailable() {
		return ErrSecureStoreBroken
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	cmd := exec.CommandContext(ctx, "security",
		"add-generic-password",
		"-U",
		"-s", darwinSecureStoreService,
		"-a", installID,
		"-w", encoded,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "User interaction is not allowed") {
			return ErrSecureStoreUnavailable
		}
		return ErrSecureStoreBroken
	}
	return nil
}

func (s *darwinSecureStore) LoadDeviceKey(ctx context.Context, installID string) ([]byte, error) {
	if !s.Capability(ctx).IsAvailable() {
		return nil, ErrSecureStoreBroken
	}

	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password",
		"-s", darwinSecureStoreService,
		"-a", installID,
		"-w",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := string(out)
		switch {
		case strings.Contains(message, "could not be found"):
			return nil, ErrSecureStoreNotFound
		case strings.Contains(message, "User interaction is not allowed"):
			return nil, ErrSecureStoreUnavailable
		default:
			return nil, ErrSecureStoreBroken
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, ErrSecureStoreBroken
	}
	return decoded, nil
}

func (s *darwinSecureStore) DeleteDeviceKey(ctx context.Context, installID string) error {
	if !s.Capability(ctx).IsAvailable() {
		return ErrSecureStoreBroken
	}

	cmd := exec.CommandContext(ctx, "security",
		"delete-generic-password",
		"-s", darwinSecureStoreService,
		"-a", installID,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if strings.Contains(string(out), "could not be found") {
		return nil
	}
	if strings.Contains(string(out), "User interaction is not allowed") {
		return ErrSecureStoreUnavailable
	}
	return ErrSecureStoreBroken
}
