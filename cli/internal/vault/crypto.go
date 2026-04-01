package vault

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	SaltSize = 32
	KeySize  = chacha20poly1305.KeySize // 32
	NonceSize = chacha20poly1305.NonceSizeX // 24
)

type KDFParams struct {
	Salt        [SaltSize]byte
	TimeCost    uint32
	MemoryCost  uint32 // in KiB
	Parallelism uint8
}

func DefaultKDFParams() KDFParams {
	var salt [SaltSize]byte
	if _, err := rand.Read(salt[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return KDFParams{
		Salt:        salt,
		TimeCost:    3,
		MemoryCost:  64 * 1024, // 64 MB
		Parallelism: 4,
	}
}

func DeriveKey(password []byte, params KDFParams) []byte {
	return argon2.IDKey(
		password,
		params.Salt[:],
		params.TimeCost,
		params.MemoryCost,
		params.Parallelism,
		KeySize,
	)
}

func Encrypt(key, plaintext []byte) (nonce []byte, ciphertext []byte, err error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cipher: %w", err)
	}

	nonce = make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext = aead.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password or corrupted vault): %w", err)
	}

	return plaintext, nil
}
