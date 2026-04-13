package vault

import (
	"fmt"
	"os"
)

func VerifyPassword(path string, password []byte) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading vault: %w", err)
	}

	header, _, err := UnmarshalVault(data)
	if err != nil {
		return err
	}

	masterKey := DeriveKey(password, header.KDF)
	defer zeroBytes(masterKey)

	stretchedKey, err := DeriveStretchedKey(masterKey)
	if err != nil {
		return fmt.Errorf("deriving stretched key: %w", err)
	}
	defer zeroBytes(stretchedKey)

	if _, err := DecryptCombined(stretchedKey, header.ProtectedKey[:]); err != nil {
		return fmt.Errorf("invalid master password")
	}

	return nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
