package sensitiveauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

	deviceKey, err := recoverLocalEnrollmentDeviceKey(paths, enrollment, installID)
	if err != nil {
		return nil, err
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
		if isHeadlessLocalUnlockAllowed(capability) {
			return refreshHeadlessLocalEnrollment(paths, symmetricKey, capability)
		}
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
		TrustMode:                LocalEnrollmentTrustSecureStore,
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
	_ = os.Remove(paths.HeadlessUnlockKeyFile())

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

func recoverLocalEnrollmentDeviceKey(paths config.Paths, enrollment *LocalEnrollment, installID string) ([]byte, error) {
	if enrollment != nil && enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile {
		deviceKey, err := os.ReadFile(paths.HeadlessUnlockKeyFile())
		if err != nil {
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Reading headless local unlock key: %w", err))
		}
		if len(deviceKey) != vault.KeySize {
			zeroSensitiveBytes(deviceKey)
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Headless local unlock key is invalid"))
		}
		return deviceKey, nil
	}

	store := NewSecureStore()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deviceKey, err := store.LoadDeviceKey(ctx, installID)
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Loading secure-store device key: %w", err))
	}
	return deviceKey, nil
}

func refreshHeadlessLocalEnrollment(paths config.Paths, symmetricKey []byte, capability CapabilityState) (EnrollmentResult, error) {
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
			Reason:     fmt.Sprintf("generating headless device key: %v", err),
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

	if err := writeHeadlessUnlockKey(paths.HeadlessUnlockKeyFile(), deviceKey); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}

	enrollment := LocalEnrollment{
		Version:                  LocalEnrollmentVersion,
		TrustMode:                LocalEnrollmentTrustHeadlessFile,
		InstallID:                installID,
		LocalUser:                CurrentLocalUser(),
		CreatedAt:                time.Now().UTC(),
		WrappedVaultSymmetricKey: wrappedVaultKey,
	}
	if err := WriteLocalEnrollment(paths.LocalUnlockBlobFile(), enrollment); err != nil {
		_ = os.Remove(paths.HeadlessUnlockKeyFile())
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}

	return EnrollmentResult{
		Refreshed:  true,
		Capability: capability,
	}, nil
}

func writeHeadlessUnlockKey(path string, key []byte) error {
	if len(key) != vault.KeySize {
		return fmt.Errorf("Headless local unlock key is invalid")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating headless local unlock directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "headless-unlock-*.tmp")
	if err != nil {
		return fmt.Errorf("Creating headless local unlock temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(key); err != nil {
		tmp.Close()
		return fmt.Errorf("Writing headless local unlock key: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("Setting headless local unlock permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing headless local unlock temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("Replacing headless local unlock key: %w", err)
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
	if enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile {
		return false
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

func isHeadlessLocalUnlockAllowed(capability CapabilityState) bool {
	if !capability.IsUnavailable() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("FORGED_HEADLESS")), "1") {
		return true
	}
	return runtime.GOOS == "linux"
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
