package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// LoadOrCreateInstallID returns the stable per-installation ID shared by
// account credentials and local-unlock enrollment. The common lock prevents
// concurrent first use from publishing two different IDs.
func LoadOrCreateInstallID(paths Paths) (string, error) {
	var installID string
	if err := withConfigLock(paths.InstallIDFile(), func() error {
		if data, err := os.ReadFile(paths.InstallIDFile()); err == nil {
			if installID = strings.TrimSpace(string(data)); installID != "" {
				return nil
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("reading install ID: %w", err)
		}

		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return fmt.Errorf("generating install ID: %w", err)
		}
		installID = hex.EncodeToString(raw)
		if err := writePrivateFileAtomicDurable(paths.InstallIDFile(), []byte(installID+"\n")); err != nil {
			return fmt.Errorf("writing install ID: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}
	return installID, nil
}
