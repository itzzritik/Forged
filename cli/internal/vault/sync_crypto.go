package vault

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

func DeriveSyncKey(vaultKey []byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, vaultKey, nil, []byte("forged-sync"))
	syncKey := make([]byte, KeySize)
	if _, err := hkdfReader.Read(syncKey); err != nil {
		return nil, fmt.Errorf("deriving sync key: %w", err)
	}
	return syncKey, nil
}

func EncryptForSync(vaultKey, plaintext []byte) ([]byte, error) {
	syncKey, err := DeriveSyncKey(vaultKey)
	if err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.NewX(syncKey)
	if err != nil {
		return nil, fmt.Errorf("creating sync cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating sync nonce: %w", err)
	}

	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func DecryptFromSync(vaultKey, data []byte) ([]byte, error) {
	syncKey, err := DeriveSyncKey(vaultKey)
	if err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.NewX(syncKey)
	if err != nil {
		return nil, fmt.Errorf("creating sync cipher: %w", err)
	}

	if len(data) < aead.NonceSize() {
		return nil, fmt.Errorf("sync data too short")
	}

	nonce := data[:aead.NonceSize()]
	ciphertext := data[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("sync decryption failed: %w", err)
	}

	return plaintext, nil
}
