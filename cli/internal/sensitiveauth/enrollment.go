package sensitiveauth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
)

const (
	LocalEnrollmentVersion = 2

	LocalEnrollmentTrustSecureStore  = "secure_store"
	LocalEnrollmentTrustHeadlessFile = "headless_file"

	localEnrollmentDeviceKeySlotA = "a"
	localEnrollmentDeviceKeySlotB = "b"
)

type LocalEnrollment struct {
	Version                  int       `json:"version"`
	TrustMode                string    `json:"trust_mode,omitempty"`
	InstallID                string    `json:"install_id"`
	LocalUser                string    `json:"local_user,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	ExpiresAt                time.Time `json:"expires_at"`
	LastUsedAt               time.Time `json:"last_used_at,omitempty"`
	DeviceKeySlot            string    `json:"device_key_slot,omitempty"`
	Revoked                  bool      `json:"revoked,omitempty"`
	WrappedVaultSymmetricKey []byte    `json:"wrapped_vault_symmetric_key"`
}

func ReadLocalEnrollment(path string) (*LocalEnrollment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var enrollment LocalEnrollment
	if err := json.Unmarshal(data, &enrollment); err != nil {
		return nil, fmt.Errorf("Parsing local enrollment: %w", err)
	}
	if enrollment.Revoked {
		return nil, os.ErrNotExist
	}
	return &enrollment, nil
}

func WriteLocalEnrollment(path string, enrollment LocalEnrollment) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating local enrollment directory: %w", err)
	}

	body, err := json.MarshalIndent(enrollment, "", "  ")
	if err != nil {
		return fmt.Errorf("Serializing local enrollment: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "local-unlock-*.tmp")
	if err != nil {
		return fmt.Errorf("Creating local enrollment temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("Writing local enrollment: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("Setting local enrollment permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing local enrollment: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing local enrollment temp file: %w", err)
	}
	if err := replaceLocalEnrollmentFile(tmpPath, path); err != nil {
		return fmt.Errorf("Replacing local enrollment: %w", err)
	}
	if err := syncLocalEnrollmentDir(path); err != nil {
		return err
	}
	return nil
}

func DeleteLocalEnrollment(path string) error {
	// Keep a durable tombstone instead of relying on an unlink. Invalidation
	// deletes device-key slots afterwards, so a crash must never resurrect an
	// old metadata pointer that still names one of those slots.
	if err := WriteLocalEnrollment(path, LocalEnrollment{Revoked: true}); err != nil {
		return fmt.Errorf("revoking local enrollment: %w", err)
	}
	return nil
}

func withLocalEnrollmentLock(paths config.Paths, fn func() error) error {
	if err := os.MkdirAll(paths.AuthDir(), 0o700); err != nil {
		return fmt.Errorf("creating local enrollment directory: %w", err)
	}
	lock, err := os.OpenFile(paths.LocalUnlockLockFile(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening local enrollment lock: %w", err)
	}
	defer lock.Close()
	if err := platform.LockFileWait(lock); err != nil {
		return fmt.Errorf("locking local enrollment: %w", err)
	}
	defer platform.UnlockFile(lock)
	return fn()
}

func syncLocalEnrollmentDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("opening local enrollment directory: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("syncing local enrollment directory: %w", err)
	}
	return nil
}

func localEnrollmentDeviceKeySlot(enrollment *LocalEnrollment) (string, error) {
	if enrollment == nil {
		return "", nil
	}
	slot := strings.TrimSpace(enrollment.DeviceKeySlot)
	if slot == "" {
		if enrollment.Version >= LocalEnrollmentVersion {
			return "", fmt.Errorf("local enrollment device key slot is missing")
		}
		return "", nil // legacy single-key enrollment
	}
	switch slot {
	case localEnrollmentDeviceKeySlotA, localEnrollmentDeviceKeySlotB:
		return slot, nil
	default:
		return "", fmt.Errorf("local enrollment device key slot is invalid")
	}
}

func nextLocalEnrollmentDeviceKeySlot(enrollment *LocalEnrollment) string {
	slot, err := localEnrollmentDeviceKeySlot(enrollment)
	if err == nil && slot == localEnrollmentDeviceKeySlotA {
		return localEnrollmentDeviceKeySlotB
	}
	return localEnrollmentDeviceKeySlotA
}

func headlessUnlockKeyFile(paths config.Paths, slot string) (string, error) {
	if slot == "" {
		return paths.HeadlessUnlockKeyFile(), nil
	}
	if slot != localEnrollmentDeviceKeySlotA && slot != localEnrollmentDeviceKeySlotB {
		return "", fmt.Errorf("headless local unlock key slot is invalid")
	}
	return paths.HeadlessUnlockKeySlotFile(slot), nil
}

func CurrentLocalUser() string {
	if u, err := user.Current(); err == nil {
		switch {
		case strings.TrimSpace(u.Username) != "":
			return u.Username
		case strings.TrimSpace(u.Name) != "":
			return u.Name
		}
	}
	if v := strings.TrimSpace(os.Getenv("USER")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("USERNAME")); v != "" {
		return v
	}
	return ""
}
