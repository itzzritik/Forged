package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
)

const (
	SaltSize  = 32
	KeySize   = 32
	NonceSize = 12
)

type KDFParams struct {
	Salt        [SaltSize]byte
	TimeCost    uint32
	MemoryCost  uint32
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
		MemoryCost:  64 * 1024,
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
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext = aead.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password or corrupted vault): %w", err)
	}

	return plaintext, nil
}

func DeriveStretchedKey(masterKey []byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, masterKey, nil, []byte("forged-stretch"))
	stretched := make([]byte, KeySize)
	if _, err := hkdfReader.Read(stretched); err != nil {
		return nil, fmt.Errorf("deriving stretched key: %w", err)
	}
	return stretched, nil
}

func EncryptCombined(key, plaintext []byte) ([]byte, error) {
	nonce, ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		return nil, err
	}
	combined := make([]byte, len(nonce)+len(ciphertext))
	copy(combined, nonce)
	copy(combined[len(nonce):], ciphertext)
	return combined, nil
}

func DecryptCombined(key, data []byte) ([]byte, error) {
	if len(data) < NonceSize {
		return nil, fmt.Errorf("data too short for decryption")
	}
	return Decrypt(key, data[:NonceSize], data[NonceSize:])
}
