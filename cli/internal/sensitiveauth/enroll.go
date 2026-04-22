package sensitiveauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

const localEnrollmentKeyInfo = "forged-local-enrollment"

var ErrLocalUnlockTrustUnavailable = errors.New("Local unlock trust unavailable")

type EnrollmentResult struct {
	Refreshed  bool
	Capability CapabilityState
	Reason     string
}

func RecoverEnrolledSymmetricKey(paths config.Paths) ([]byte, error) {
	enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Reading local unlock enrollment: %w", err))
	}
	if enrollmentExpired(paths, enrollment) {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Local unlock enrollment expired"))
	}

	installID, err := osReadTrimmed(paths.InstallIDFile())
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Reading install ID: %w", err))
	}
	if enrollment.InstallID == "" || enrollment.InstallID != installID {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Local unlock enrollment install ID mismatch"))
	}
	if expectedUser := strings.TrimSpace(enrollment.LocalUser); expectedUser != "" && expectedUser != CurrentLocalUser() {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Local unlock enrollment user mismatch"))
	}

	store := NewSecureStore()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deviceKey, err := store.LoadDeviceKey(ctx, installID)
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Loading secure-store device key: %w", err))
	}
	defer zeroSensitiveBytes(deviceKey)

	localKey, err := deriveLocalEnrollmentKey(deviceKey)
	if err != nil {
		return nil, err
	}
	defer zeroSensitiveBytes(localKey)

	symmetricKey, err := vault.DecryptCombined(localKey, enrollment.WrappedVaultSymmetricKey)
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Unwrapping local vault key: %w", err))
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
		return EnrollmentResult{}, fmt.Errorf("Vault symmetric key required")
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
		ExpiresAt:                time.Now().UTC().Add(config.MasterPasswordIntervalDuration(loadMasterPasswordInterval(paths))),
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
		return nil, fmt.Errorf("Deriving local enrollment key: %w", err)
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

func loadMasterPasswordInterval(paths config.Paths) string {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		return config.MasterPasswordInterval7Days
	}
	return config.NormalizeMasterPasswordInterval(cfg.Security.MasterPasswordInterval)
}

func enrollmentExpired(paths config.Paths, enrollment *LocalEnrollment) bool {
	if enrollment == nil {
		return true
	}
	if !enrollment.CreatedAt.IsZero() {
		expiry := enrollment.CreatedAt.Add(config.MasterPasswordIntervalDuration(loadMasterPasswordInterval(paths)))
		return time.Now().UTC().After(expiry)
	}
	if !enrollment.ExpiresAt.IsZero() {
		return time.Now().UTC().After(enrollment.ExpiresAt)
	}
	return false
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
