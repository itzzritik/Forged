package sensitiveauth

import (
	"fmt"
	"log/slog"

	"github.com/itzzritik/forged/cli/internal/config"
)

type PasswordVerifier struct {
	paths  config.Paths
	logger *slog.Logger
}

func NewPasswordVerifier(paths config.Paths, logger *slog.Logger) *PasswordVerifier {
	return &PasswordVerifier{paths: paths, logger: logger}
}

func (v *PasswordVerifier) Verify(password []byte) error {
	if len(password) == 0 {
		return fmt.Errorf("master password required")
	}
	result, err := VerifyAndRefreshLocalEnrollment(v.paths, password)
	if err != nil {
		return fmt.Errorf("authentication failed")
	}
	if !result.Refreshed && v.logger != nil && result.Reason != "" {
		v.logger.Warn("local unlock enrollment not refreshed", "capability", result.Capability, "reason", result.Reason)
	}
	return nil
}
