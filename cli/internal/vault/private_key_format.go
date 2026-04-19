package vault

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/keytypes"
	"golang.org/x/crypto/ssh"
)

type PrivateKeyFormat string

const (
	PrivateKeyFormatOpenSSH   PrivateKeyFormat = "openssh"
	PrivateKeyFormatPKCS8PEM  PrivateKeyFormat = "pkcs8-pem"
	PrivateKeyFormatLegacyPEM PrivateKeyFormat = "legacy-pem"
	PrivateKeyFormatUnknown   PrivateKeyFormat = "unknown"
)

var ErrPassphraseProtectedPrivateKey = errors.New("passphrase-protected private keys are not supported")

type NormalizedPrivateKey struct {
	Bytes       []byte
	Converted   bool
	Fingerprint string
	Format      PrivateKeyFormat
	PublicKey   string
	Type        string
}

func normalizePrivateKeyPEM(input []byte) []byte {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 {
		return nil
	}
	return append(append([]byte(nil), trimmed...), '\n')
}

func DetectPrivateKeyFormat(input []byte) PrivateKeyFormat {
	block, _ := pem.Decode(bytes.TrimSpace(input))
	if block == nil {
		return PrivateKeyFormatUnknown
	}
	if block.Type == "OPENSSH PRIVATE KEY" {
		return PrivateKeyFormatOpenSSH
	}
	if block.Type == "PRIVATE KEY" || block.Type == "ENCRYPTED PRIVATE KEY" {
		return PrivateKeyFormatPKCS8PEM
	}
	if block.Type == "RSA PRIVATE KEY" || block.Type == "EC PRIVATE KEY" {
		return PrivateKeyFormatLegacyPEM
	}
	if strings.HasSuffix(block.Type, "PRIVATE KEY") {
		return PrivateKeyFormatPKCS8PEM
	}
	return PrivateKeyFormatUnknown
}

func NormalizePrivateKeyToOpenSSH(input []byte, comment string) (NormalizedPrivateKey, error) {
	normalizedInput := normalizePrivateKeyPEM(input)
	if len(normalizedInput) == 0 {
		return NormalizedPrivateKey{}, fmt.Errorf("private key is empty")
	}

	format := DetectPrivateKeyFormat(normalizedInput)

	rawKey, err := ssh.ParseRawPrivateKey(normalizedInput)
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return NormalizedPrivateKey{}, ErrPassphraseProtectedPrivateKey
		}
		return NormalizedPrivateKey{}, fmt.Errorf("parsing private key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(rawKey)
	if err != nil {
		return NormalizedPrivateKey{}, fmt.Errorf("creating signer from private key: %w", err)
	}

	storedBytes := append([]byte(nil), normalizedInput...)
	converted := false

	if format != PrivateKeyFormatOpenSSH {
		block, err := ssh.MarshalPrivateKey(rawKey, comment)
		if err != nil {
			return NormalizedPrivateKey{}, fmt.Errorf("converting private key to OpenSSH: %w", err)
		}
		storedBytes = pem.EncodeToMemory(block)
		converted = true

		convertedSigner, err := ssh.ParsePrivateKey(storedBytes)
		if err != nil {
			return NormalizedPrivateKey{}, fmt.Errorf("validating converted OpenSSH private key: %w", err)
		}
		if !bytes.Equal(convertedSigner.PublicKey().Marshal(), signer.PublicKey().Marshal()) {
			return NormalizedPrivateKey{}, fmt.Errorf("converted OpenSSH private key does not match original public key")
		}
	}

	return NormalizedPrivateKey{
		Bytes:       storedBytes,
		Converted:   converted,
		Fingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
		Format:      format,
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))),
		Type:        keytypes.FromSSHPublicKeyType(signer.PublicKey().Type()),
	}, nil
}
