//go:build darwin

package sensitiveauth

import (
	"context"
	"encoding/base64"
	"os/exec"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

const darwinSecureStoreService = "com.forged.local-unlock"

type darwinSecureStore struct{}

func newPlatformSecureStore(config.Paths) SecureStore {
	return &darwinSecureStore{}
}

func (s *darwinSecureStore) Capability(context.Context) CapabilityState {
	if _, err := exec.LookPath("security"); err != nil {
		return CapabilityBroken
	}
	return CapabilityAvailable
}

func (s *darwinSecureStore) SaveDeviceKey(ctx context.Context, installID, slot string, key []byte) error {
	if !s.Capability(ctx).IsAvailable() {
		return ErrSecureStoreBroken
	}
	account, err := secureStoreDeviceKeyAccount(installID, slot)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	if out, err := platform.SecurityAddGenericPassword(ctx, darwinSecureStoreService, account, encoded); err != nil {
		if strings.Contains(string(out), "User interaction is not allowed") {
			return ErrSecureStoreUnavailable
		}
		return ErrSecureStoreBroken
	}
	return nil
}

func (s *darwinSecureStore) LoadDeviceKey(ctx context.Context, installID, slot string) ([]byte, error) {
	if !s.Capability(ctx).IsAvailable() {
		return nil, ErrSecureStoreBroken
	}
	account, err := secureStoreDeviceKeyAccount(installID, slot)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password",
		"-s", darwinSecureStoreService,
		"-a", account,
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

func (s *darwinSecureStore) DeleteDeviceKey(ctx context.Context, installID, slot string) error {
	if !s.Capability(ctx).IsAvailable() {
		return ErrSecureStoreBroken
	}
	account, err := secureStoreDeviceKeyAccount(installID, slot)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "security",
		"delete-generic-password",
		"-s", darwinSecureStoreService,
		"-a", account,
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
