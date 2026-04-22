package sensitiveauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

const localEnrollmentKeyInfo = "forged-local-enrollment"

type EnrollmentResult struct {
	Refreshed  bool
	Capability CapabilityState
	Reason     string
}

func RecoverEnrolledSymmetricKey(paths config.Paths) ([]byte, error) {
	enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	if err != nil {
		return nil, fmt.Errorf("reading local unlock enrollment: %w", err)
	}
	if time.Now().UTC().After(enrollment.ExpiresAt) {
		return nil, fmt.Errorf("local unlock enrollment expired")
	}

	installID, err := osReadTrimmed(paths.InstallIDFile())
	if err != nil {
		return nil, fmt.Errorf("reading install id: %w", err)
	}
	if enrollment.InstallID == "" || enrollment.InstallID != installID {
		return nil, fmt.Errorf("local unlock enrollment install id mismatch")
	}
	if expectedUser := strings.TrimSpace(enrollment.LocalUser); expectedUser != "" && expectedUser != CurrentLocalUser() {
		return nil, fmt.Errorf("local unlock enrollment user mismatch")
	}

	store := NewSecureStore()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deviceKey, err := store.LoadDeviceKey(ctx, installID)
	if err != nil {
		return nil, fmt.Errorf("loading secure-store device key: %w", err)
	}
	defer zeroSensitiveBytes(deviceKey)

	localKey, err := deriveLocalEnrollmentKey(deviceKey)
	if err != nil {
		return nil, err
	}
	defer zeroSensitiveBytes(localKey)

	symmetricKey, err := vault.DecryptCombined(localKey, enrollment.WrappedVaultSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("unwrapping local vault key: %w", err)
	}
	return symmetricKey, nil
}

func HasLocalEnrollment(paths config.Paths) bool {
	if _, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile()); err != nil {
		return false
	}
	installID, err := osReadTrimmed(paths.InstallIDFile())
	return err == nil && strings.TrimSpace(installID) != ""
}

func VerifyAndRefreshLocalEnrollment(paths config.Paths, password []byte) (EnrollmentResult, error) {
	symmetricKey, err := vault.RecoverSymmetricKey(paths.VaultFile(), password)
	if err != nil {
		return EnrollmentResult{}, err
	}
	defer zeroSensitiveBytes(symmetricKey)

	return RefreshLocalEnrollment(paths, symmetricKey)
}

func RefreshLocalEnrollment(paths config.Paths, symmetricKey []byte) (EnrollmentResult, error) {
	if len(symmetricKey) == 0 {
		return EnrollmentResult{}, fmt.Errorf("vault symmetric key required")
	}

	store := NewSecureStore()
	capability := store.Capability(context.Background())
	if !capability.IsAvailable() {
		return EnrollmentResult{
			Capability: capability,
			Reason:     "secure storage unavailable for local unlock enrollment",
		}, nil
	}

	installID, err := LoadOrCreateInstallID(paths)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}

	deviceKey := make([]byte, vault.KeySize)
	if _, err := rand.Read(deviceKey); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("generating device key: %v", err),
		}, nil
	}
	defer zeroSensitiveBytes(deviceKey)

	localKey, err := deriveLocalEnrollmentKey(deviceKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}
	defer zeroSensitiveBytes(localKey)

	wrappedVaultKey, err := vault.EncryptCombined(localKey, symmetricKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("wrapping vault symmetric key: %v", err),
		}, nil
	}

	enrollment := LocalEnrollment{
		Version:                  LocalEnrollmentVersion,
		InstallID:                installID,
		LocalUser:                CurrentLocalUser(),
		CreatedAt:                time.Now().UTC(),
		ExpiresAt:                time.Now().UTC().Add(config.MasterPasswordIntervalDuration(loadConfig(paths))),
		WrappedVaultSymmetricKey: wrappedVaultKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.SaveDeviceKey(ctx, installID, deviceKey); err != nil {
		return EnrollmentResult{
			Capability: capabilityFromSecureStoreError(err),
			Reason:     err.Error(),
		}, nil
	}

	if err := WriteLocalEnrollment(paths.LocalUnlockBlobFile(), enrollment); err != nil {
		_ = store.DeleteDeviceKey(ctx, installID)
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}

	return EnrollmentResult{
		Refreshed:  true,
		Capability: CapabilityAvailable,
	}, nil
}

func InvalidateLocalEnrollment(paths config.Paths) error {
	_ = DeleteLocalEnrollment(paths.LocalUnlockBlobFile())

	installID, err := osReadTrimmed(paths.InstallIDFile())
	if err != nil || installID == "" {
		return nil
	}

	store := NewSecureStore()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.DeleteDeviceKey(ctx, installID); err != nil &&
		err != ErrSecureStoreUnavailable &&
		err != ErrSecureStoreNotFound &&
		err != ErrSecureStoreBroken {
		return err
	}
	return nil
}

func deriveLocalEnrollmentKey(deviceKey []byte) ([]byte, error) {
	reader := hkdf.New(sha256.New, deviceKey, nil, []byte(localEnrollmentKeyInfo))
	key := make([]byte, vault.KeySize)
	if _, err := reader.Read(key); err != nil {
		return nil, fmt.Errorf("deriving local enrollment key: %w", err)
	}
	return key, nil
}

func capabilityFromSecureStoreError(err error) CapabilityState {
	switch err {
	case nil:
		return CapabilityAvailable
	case ErrSecureStoreUnavailable:
		return CapabilityUnavailableByEnv
	default:
		return CapabilityBroken
	}
}

func loadConfig(paths config.Paths) config.Config {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return config.Config{}
	}
	return cfg
}

func osReadTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func zeroSensitiveBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
