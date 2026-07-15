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
	var symmetricKey []byte
	err := withLocalEnrollmentLock(paths, func() error {
		var err error
		symmetricKey, err = recoverEnrolledSymmetricKeyLocked(paths)
		return err
	})
	if err != nil {
		zeroSensitiveBytes(symmetricKey)
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, err)
	}
	return symmetricKey, nil
}

func recoverEnrolledSymmetricKeyLocked(paths config.Paths) ([]byte, error) {
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
	return LocalEnrollmentUsable(paths)
}

func RefreshLocalEnrollment(paths config.Paths, symmetricKey []byte) (EnrollmentResult, error) {
	if len(symmetricKey) == 0 {
		return EnrollmentResult{}, fmt.Errorf("Vault symmetric key required")
	}

	var result EnrollmentResult
	if err := withLocalEnrollmentLock(paths, func() error {
		store := NewSecureStore(paths)
		capability := store.Capability(context.Background())
		if !capability.IsAvailable() {
			if isHeadlessLocalUnlockAllowed(paths, capability) {
				result = refreshHeadlessLocalEnrollmentLocked(paths, symmetricKey, capability)
				return nil
			}
			result = EnrollmentResult{
				Capability: capability,
				Reason:     "secure storage unavailable for local unlock enrollment",
			}
			return nil
		}
		result = refreshSecureStoreLocalEnrollmentLocked(paths, symmetricKey, store, capability)
		return nil
	}); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}, nil
	}
	return result, nil
}

func refreshSecureStoreLocalEnrollmentLocked(paths config.Paths, symmetricKey []byte, store SecureStore, capability CapabilityState) EnrollmentResult {
	installID, err := config.LoadOrCreateInstallID(paths)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}

	deviceKey := make([]byte, vault.KeySize)
	if _, err := rand.Read(deviceKey); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("generating device key: %v", err),
		}
	}
	defer zeroSensitiveBytes(deviceKey)

	localKey, err := deriveLocalEnrollmentKey(deviceKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}
	defer zeroSensitiveBytes(localKey)

	wrappedVaultKey, err := vault.EncryptCombined(localKey, symmetricKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("wrapping vault symmetric key: %v", err),
		}
	}

	previous, _ := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	slot := nextLocalEnrollmentDeviceKeySlot(previous)
	enrollment := LocalEnrollment{
		Version:                  LocalEnrollmentVersion,
		TrustMode:                LocalEnrollmentTrustSecureStore,
		InstallID:                installID,
		LocalUser:                CurrentLocalUser(),
		CreatedAt:                time.Now().UTC(),
		ExpiresAt:                time.Now().UTC().Add(config.MasterPasswordIntervalDuration(loadMasterPasswordInterval(paths))),
		DeviceKeySlot:            slot,
		WrappedVaultSymmetricKey: wrappedVaultKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.SaveDeviceKey(ctx, installID, slot, deviceKey); err != nil {
		return EnrollmentResult{
			Capability: capabilityFromSecureStoreError(err),
			Reason:     err.Error(),
		}
	}

	if err := WriteLocalEnrollment(paths.LocalUnlockBlobFile(), enrollment); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}
	cleanupRetiredLocalEnrollmentKey(paths, previous, installID)

	return EnrollmentResult{
		Refreshed:  true,
		Capability: capability,
	}
}

func InvalidateLocalEnrollment(paths config.Paths) error {
	return withLocalEnrollmentLock(paths, func() error {
		if err := DeleteLocalEnrollment(paths.LocalUnlockBlobFile()); err != nil {
			return err
		}
		if err := removeHeadlessLocalEnrollmentKeys(paths); err != nil {
			return err
		}
		return removeSecureStoreLocalEnrollmentKeys(paths)
	})
}

func recoverLocalEnrollmentDeviceKey(paths config.Paths, enrollment *LocalEnrollment, installID string) ([]byte, error) {
	slot, err := localEnrollmentDeviceKeySlot(enrollment)
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, err)
	}
	if enrollment != nil && enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile {
		if !HeadlessModeEnabled(paths) {
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Headless local unlock is disabled"))
		}
		path, err := headlessUnlockKeyFile(paths, slot)
		if err != nil {
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, err)
		}
		deviceKey, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Reading headless local unlock key: %w", err))
		}
		if len(deviceKey) != vault.KeySize {
			zeroSensitiveBytes(deviceKey)
			return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Headless local unlock key is invalid"))
		}
		return deviceKey, nil
	}

	store := NewSecureStore(paths)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deviceKey, err := store.LoadDeviceKey(ctx, installID, slot)
	if err != nil {
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Loading secure-store device key: %w", err))
	}
	if len(deviceKey) != vault.KeySize {
		zeroSensitiveBytes(deviceKey)
		return nil, errors.Join(ErrLocalUnlockTrustUnavailable, fmt.Errorf("Secure-store device key is invalid"))
	}
	return deviceKey, nil
}

func refreshHeadlessLocalEnrollmentLocked(paths config.Paths, symmetricKey []byte, capability CapabilityState) EnrollmentResult {
	installID, err := config.LoadOrCreateInstallID(paths)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}

	deviceKey := make([]byte, vault.KeySize)
	if _, err := rand.Read(deviceKey); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("generating headless device key: %v", err),
		}
	}
	defer zeroSensitiveBytes(deviceKey)

	localKey, err := deriveLocalEnrollmentKey(deviceKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}
	defer zeroSensitiveBytes(localKey)

	wrappedVaultKey, err := vault.EncryptCombined(localKey, symmetricKey)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     fmt.Sprintf("wrapping vault symmetric key: %v", err),
		}
	}

	previous, _ := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	slot := nextLocalEnrollmentDeviceKeySlot(previous)
	path, err := headlessUnlockKeyFile(paths, slot)
	if err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}
	if err := writeHeadlessUnlockKey(path, deviceKey); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}

	enrollment := LocalEnrollment{
		Version:                  LocalEnrollmentVersion,
		TrustMode:                LocalEnrollmentTrustHeadlessFile,
		InstallID:                installID,
		LocalUser:                CurrentLocalUser(),
		CreatedAt:                time.Now().UTC(),
		DeviceKeySlot:            slot,
		WrappedVaultSymmetricKey: wrappedVaultKey,
	}
	if err := WriteLocalEnrollment(paths.LocalUnlockBlobFile(), enrollment); err != nil {
		return EnrollmentResult{
			Capability: CapabilityBroken,
			Reason:     err.Error(),
		}
	}
	cleanupRetiredLocalEnrollmentKey(paths, previous, installID)

	return EnrollmentResult{
		Refreshed:  true,
		Capability: capability,
	}
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
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("Syncing headless local unlock key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing headless local unlock temp file: %w", err)
	}
	if err := replaceLocalEnrollmentFile(tmpPath, path); err != nil {
		return fmt.Errorf("Replacing headless local unlock key: %w", err)
	}
	if err := syncLocalEnrollmentDir(path); err != nil {
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

// localEnrollmentHardCap forces one master-password re-verification even on a
// continuously-active device, no matter how often Touch ID slides the window.
// It is the backstop that separates "slide on use" from "never expire".
const localEnrollmentHardCap = 90 * 24 * time.Hour

func enrollmentExpired(paths config.Paths, enrollment *LocalEnrollment) bool {
	if enrollment == nil {
		return true
	}
	if enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile {
		return false
	}

	now := time.Now().UTC()
	interval := config.MasterPasswordIntervalDuration(loadMasterPasswordInterval(paths))

	// Absolute cap since the last master-password entry (CreatedAt). Guard so a
	// future interval longer than the cap can never expire before the window.
	hardCap := localEnrollmentHardCap
	if interval > hardCap {
		hardCap = interval
	}
	if !enrollment.CreatedAt.IsZero() && now.After(enrollment.CreatedAt.Add(hardCap)) {
		return true
	}

	// Sliding window: LastUsedAt is stamped on every biometric unlock, so an
	// active device only expires after a full interval with no use. Fall back to
	// CreatedAt for enrollments written before sliding existed.
	lastUsed := enrollment.LastUsedAt
	if lastUsed.IsZero() {
		lastUsed = enrollment.CreatedAt
	}
	if !lastUsed.IsZero() {
		return now.After(lastUsed.Add(interval))
	}
	if !enrollment.ExpiresAt.IsZero() {
		return now.After(enrollment.ExpiresAt)
	}
	return false
}

// renewLocalEnrollmentThrottle keeps repeated cold hydrates from rewriting the
// blob constantly; the sliding window only needs coarse "used recently" fidelity.
const renewLocalEnrollmentThrottle = time.Hour

// RenewLocalEnrollmentUsage slides the enrollment window forward after a
// successful biometric unlock. Best-effort: if the rewrite fails the only cost
// is an earlier expiry, never lost access, so errors are swallowed.
func RenewLocalEnrollmentUsage(paths config.Paths) {
	_ = withLocalEnrollmentLock(paths, func() error {
		renewLocalEnrollmentUsageLocked(paths)
		return nil
	})
}

func renewLocalEnrollmentUsageLocked(paths config.Paths) {
	enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	if err != nil || enrollment == nil {
		return
	}
	if enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile {
		return // headless never expires; nothing to slide
	}
	now := time.Now().UTC()
	if !enrollment.LastUsedAt.IsZero() && now.Sub(enrollment.LastUsedAt) < renewLocalEnrollmentThrottle {
		return
	}
	enrollment.LastUsedAt = now
	_ = WriteLocalEnrollment(paths.LocalUnlockBlobFile(), *enrollment)
}

// LocalEnrollmentUsable reports whether a non-expired device-unlock enrollment
// exists for this install, without touching the secure store or biometrics. The
// broker uses it to skip a doomed Touch ID prompt and fall through to the
// master-password popup once the sliding window has lapsed.
func LocalEnrollmentUsable(paths config.Paths) bool {
	usable := false
	if err := withLocalEnrollmentLock(paths, func() error {
		usable = localEnrollmentUsableLocked(paths)
		return nil
	}); err != nil {
		return false
	}
	return usable
}

func localEnrollmentUsableLocked(paths config.Paths) bool {
	enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
	if err != nil || enrollment == nil {
		return false
	}
	if _, err := localEnrollmentDeviceKeySlot(enrollment); err != nil {
		return false
	}
	if enrollmentExpired(paths, enrollment) {
		return false
	}
	if enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile && !HeadlessModeEnabled(paths) {
		return false
	}
	installID, err := osReadTrimmed(paths.InstallIDFile())
	if err != nil || installID == "" || enrollment.InstallID != installID {
		return false
	}
	if expectedUser := strings.TrimSpace(enrollment.LocalUser); expectedUser != "" && expectedUser != CurrentLocalUser() {
		return false
	}
	return true
}

func HeadlessModeSupported(paths config.Paths) bool {
	return runtime.GOOS == "linux" && NewSecureStore(paths).Capability(context.Background()).IsUnavailable()
}

func HeadlessModeEnabled(paths config.Paths) bool {
	if !HeadlessModeSupported(paths) {
		return false
	}
	return config.HeadlessUnlockEnabled(paths)
}

func headlessModePreemptsSystemAuth(paths config.Paths) bool {
	if !HeadlessModeEnabled(paths) {
		return false
	}
	preempts := false
	if err := withLocalEnrollmentLock(paths, func() error {
		enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
		preempts = err != nil || enrollment.TrustMode == LocalEnrollmentTrustHeadlessFile
		return nil
	}); err != nil {
		return false
	}
	return preempts
}

func InvalidateHeadlessEnrollment(paths config.Paths) error {
	return withLocalEnrollmentLock(paths, func() error {
		enrollment, err := ReadLocalEnrollment(paths.LocalUnlockBlobFile())
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("Reading local enrollment: %w", err)
		}
		if enrollment.TrustMode != LocalEnrollmentTrustHeadlessFile {
			return nil
		}
		if err := DeleteLocalEnrollment(paths.LocalUnlockBlobFile()); err != nil {
			return err
		}
		return removeHeadlessLocalEnrollmentKeys(paths)
	})
}

func isHeadlessLocalUnlockAllowed(paths config.Paths, capability CapabilityState) bool {
	if !capability.IsUnavailable() {
		return false
	}
	return HeadlessModeEnabled(paths)
}

var localEnrollmentDeviceKeySlots = []string{
	"",
	localEnrollmentDeviceKeySlotA,
	localEnrollmentDeviceKeySlotB,
}

func cleanupRetiredLocalEnrollmentKey(paths config.Paths, previous *LocalEnrollment, fallbackInstallID string) {
	// Refresh calls this only after both the inactive key and its metadata
	// pointer reached their durable commit points.
	if previous == nil {
		return
	}
	slot, err := localEnrollmentDeviceKeySlot(previous)
	if err != nil {
		return
	}
	if previous.TrustMode == LocalEnrollmentTrustHeadlessFile {
		path, err := headlessUnlockKeyFile(paths, slot)
		if err == nil {
			_ = os.Remove(path)
		}
		return
	}
	installID := strings.TrimSpace(previous.InstallID)
	if installID == "" {
		installID = fallbackInstallID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = NewSecureStore(paths).DeleteDeviceKey(ctx, installID, slot)
}

func removeHeadlessLocalEnrollmentKeys(paths config.Paths) error {
	for _, slot := range localEnrollmentDeviceKeySlots {
		path, err := headlessUnlockKeyFile(paths, slot)
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Deleting headless unlock key: %w", err)
		}
	}
	return nil
}

func removeSecureStoreLocalEnrollmentKeys(paths config.Paths) error {
	installID, _ := osReadTrimmed(paths.InstallIDFile())
	store := NewSecureStore(paths)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, slot := range localEnrollmentDeviceKeySlots {
		err := store.DeleteDeviceKey(ctx, installID, slot)
		if err == nil || errors.Is(err, ErrSecureStoreUnavailable) ||
			errors.Is(err, ErrSecureStoreNotFound) || errors.Is(err, ErrSecureStoreBroken) {
			continue
		}
		return err
	}
	return nil
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
