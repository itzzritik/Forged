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
