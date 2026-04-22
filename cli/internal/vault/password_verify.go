package vault

import (
	"fmt"
	"os"
)

func VerifyPassword(path string, password []byte) error {
	symmetricKey, err := RecoverSymmetricKey(path, password)
	if err != nil {
		return err
	}
	zeroBytes(symmetricKey)
	return nil
}

func RecoverSymmetricKey(path string, password []byte) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Reading vault: %w", err)
	}

	header, _, err := UnmarshalVault(data)
	if err != nil {
		return nil, err
	}

	masterKey := DeriveKey(password, header.KDF)
	defer zeroBytes(masterKey)

	stretchedKey, err := DeriveStretchedKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("Deriving stretched key: %w", err)
	}
	defer zeroBytes(stretchedKey)

	symmetricKey, err := DecryptCombined(stretchedKey, header.ProtectedKey[:])
	if err != nil {
		return nil, fmt.Errorf("Invalid master password")
	}

	return symmetricKey, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
