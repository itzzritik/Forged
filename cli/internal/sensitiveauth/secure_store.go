package sensitiveauth

import (
	"context"
	"errors"
	"strings"

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

func secureStoreDeviceKeyAccount(installID, slot string) (string, error) {
	installID = strings.TrimSpace(installID)
	if installID == "" {
		return "", ErrSecureStoreBroken
	}
	switch slot {
	case "":
		return installID, nil // legacy single-key enrollment
	case localEnrollmentDeviceKeySlotA, localEnrollmentDeviceKeySlotB:
		return installID + ".v2." + slot, nil
	default:
		return "", ErrSecureStoreBroken
	}
}
