package sensitiveauth

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type PasswordVerifier struct {
	paths config.Paths
}

func NewPasswordVerifier(paths config.Paths) *PasswordVerifier {
	return &PasswordVerifier{paths: paths}
}

func (v *PasswordVerifier) Verify(password []byte) error {
	if len(password) == 0 {
		return fmt.Errorf("Master password required")
	}
	if err := vault.VerifyPassword(v.paths.VaultFile(), password); err != nil {
		return fmt.Errorf("Authentication failed")
	}
	return nil
}
