package sensitiveauth

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type PasswordVerifier struct {
	vaultPath string
}

func NewPasswordVerifier(vaultPath string) *PasswordVerifier {
	return &PasswordVerifier{vaultPath: vaultPath}
}

func (v *PasswordVerifier) Verify(password []byte) error {
	if len(password) == 0 {
		return fmt.Errorf("master password required")
	}
	if err := vault.VerifyPassword(v.vaultPath, password); err != nil {
		return fmt.Errorf("authentication failed")
	}
	return nil
}
