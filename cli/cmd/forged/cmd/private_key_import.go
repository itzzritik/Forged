package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/vault"
)

func formatPrivateKeyImportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, vault.ErrPassphraseProtectedPrivateKey) {
		return fmt.Errorf("key is passphrase-protected. Remove the passphrase first with: ssh-keygen -p -f <file>")
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "private key is empty"):
		return fmt.Errorf("private key is empty")
	case strings.Contains(msg, "parsing private key"):
		return fmt.Errorf("unrecognized key format. Only unencrypted OpenSSH or PEM private keys are supported")
	case strings.Contains(msg, "creating signer from private key"):
		return fmt.Errorf("unsupported private key type. Only RSA, ECDSA, and Ed25519 keys are supported")
	default:
		return err
	}
}

func normalizeImportedKeyType(raw string) string {
	switch {
	case strings.Contains(raw, "ed25519"):
		return "ed25519"
	case strings.Contains(raw, "ecdsa"):
		return "ecdsa"
	case strings.Contains(raw, "rsa"):
		return "rsa"
	default:
		return "unknown"
	}
}

func privateKeyImportFormatLabel(format vault.PrivateKeyFormat) string {
	switch format {
	case vault.PrivateKeyFormatOpenSSH:
		return "OpenSSH"
	case vault.PrivateKeyFormatPKCS8PEM:
		return "PKCS#8 PEM -> OpenSSH"
	case vault.PrivateKeyFormatLegacyPEM:
		return "Legacy PEM -> OpenSSH"
	default:
		return "PEM -> OpenSSH"
	}
}

func privateKeyConversionSummary(formats []vault.PrivateKeyFormat) (string, bool) {
	pkcs8Count := 0
	legacyCount := 0

	for _, format := range formats {
		switch format {
		case vault.PrivateKeyFormatPKCS8PEM:
			pkcs8Count++
		case vault.PrivateKeyFormatLegacyPEM:
			legacyCount++
		}
	}

	total := pkcs8Count + legacyCount
	if total == 0 {
		return "", false
	}

	switch {
	case pkcs8Count > 0 && legacyCount == 0:
		return fmt.Sprintf("  Warning: %d PKCS#8 PEM key(s) will be converted to the latest OpenSSH private key format.\n  The underlying keypair stays the same, so existing GitHub/server setups continue to work.", pkcs8Count), true
	case legacyCount > 0 && pkcs8Count == 0:
		return fmt.Sprintf("  Warning: %d legacy PEM key(s) will be converted to the latest OpenSSH private key format.\n  The underlying keypair stays the same, so existing GitHub/server setups continue to work.", legacyCount), true
	default:
		return fmt.Sprintf("  Warning: %d PEM key(s) will be converted to the latest OpenSSH private key format.\n  This includes %d PKCS#8 PEM key(s) and %d legacy PEM key(s). The underlying keypair stays the same, so existing GitHub/server setups continue to work.", total, pkcs8Count, legacyCount), true
	}
}

func singlePrivateKeyConversionWarning(format vault.PrivateKeyFormat) string {
	switch format {
	case vault.PrivateKeyFormatPKCS8PEM:
		return "  Warning: PKCS#8 PEM detected. Forged will convert it to the latest OpenSSH private key format.\n  The keypair stays the same, so existing GitHub/server setups continue to work."
	case vault.PrivateKeyFormatLegacyPEM:
		return "  Warning: legacy PEM detected. Forged will convert it to the latest OpenSSH private key format.\n  The keypair stays the same, so existing GitHub/server setups continue to work."
	default:
		return ""
	}
}
