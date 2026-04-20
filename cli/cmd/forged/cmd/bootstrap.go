package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

func createVaultAtPaths(paths config.Paths, password []byte) (*vault.Vault, *vault.KeyStore, error) {
	v, err := vault.Create(paths.VaultFile(), password)
	if err != nil {
		return nil, nil, fmt.Errorf("creating vault: %w", err)
	}

	return v, vault.NewKeyStore(v), nil
}
