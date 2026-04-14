package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/vault"
)

func ensureDefaultConfig(paths config.Paths) error {
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o700); err != nil {
		return err
	}

	if _, err := os.Stat(paths.ConfigFile()); err == nil {
		return nil
	}

	content := fmt.Sprintf(`[agent]
socket = %q
log_level = "info"

[sync]
enabled = false
`, paths.AgentSocket())

	return os.WriteFile(paths.ConfigFile(), []byte(content), 0o600)
}

func createVaultAtPaths(paths config.Paths, password []byte) (*vault.Vault, *vault.KeyStore, error) {
	v, err := vault.Create(paths.VaultFile(), password)
	if err != nil {
		return nil, nil, fmt.Errorf("creating vault: %w", err)
	}

	return v, vault.NewKeyStore(v), nil
}

func ensureLocalService(paths config.Paths, password []byte) error {
	if err := daemon.InstallService(paths, string(password)); err != nil {
		return err
	}
	return daemon.StartService()
}
