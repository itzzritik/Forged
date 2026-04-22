package sensitiveauth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

const LocalEnrollmentVersion = 1

type LocalEnrollment struct {
	Version                  int       `json:"version"`
	InstallID                string    `json:"install_id"`
	LocalUser                string    `json:"local_user,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	ExpiresAt                time.Time `json:"expires_at"`
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
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing local enrollment temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("Replacing local enrollment: %w", err)
	}
	return nil
}

func DeleteLocalEnrollment(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Deleting local enrollment: %w", err)
	}
	return nil
}

func LoadOrCreateInstallID(paths config.Paths) (string, error) {
	if data, err := os.ReadFile(paths.InstallIDFile()); err == nil {
		if installID := strings.TrimSpace(string(data)); installID != "" {
			return installID, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("Reading install ID: %w", err)
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("Generating install ID: %w", err)
	}
	installID := hex.EncodeToString(raw)

	if err := os.MkdirAll(filepath.Dir(paths.InstallIDFile()), 0o700); err != nil {
		return "", fmt.Errorf("Creating install ID directory: %w", err)
	}
	if err := os.WriteFile(paths.InstallIDFile(), []byte(installID+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("Writing install ID: %w", err)
	}
	return installID, nil
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
