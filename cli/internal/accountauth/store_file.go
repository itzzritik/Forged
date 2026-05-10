package accountauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
)

type encryptedSecretFile struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type fileCredentialStore struct {
	paths config.Paths
}

func newFileCredentialStore(paths config.Paths) credentialStore {
	return fileCredentialStore{paths: paths}
}

func (s fileCredentialStore) Backend() string { return backendEncryptedFile }

func (s fileCredentialStore) Available(context.Context) bool { return true }

func (s fileCredentialStore) Save(_ context.Context, credentialID string, secret credentialSecret) error {
	key, err := s.loadOrCreateKey()
	if err != nil {
		return err
	}
	defer zeroBytes(key)

	plaintext, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("Serializing account secret: %w", err)
	}
	defer zeroBytes(plaintext)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("Creating account secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("Creating account secret cipher mode: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("Generating account secret nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(credentialID))

	body, err := json.MarshalIndent(encryptedSecretFile{
		Version:    1,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("Serializing encrypted account secret: %w", err)
	}
	if err := writePrivateFile(s.paths.AccountSecretFile(), body); err != nil {
		return fmt.Errorf("Writing encrypted account secret: %w", err)
	}
	return nil
}

func (s fileCredentialStore) Load(_ context.Context, credentialID string) (credentialSecret, error) {
	data, err := os.ReadFile(s.paths.AccountSecretFile())
	if err != nil {
		if os.IsNotExist(err) {
			return credentialSecret{}, ErrCredentialSecretNotFound
		}
		return credentialSecret{}, err
	}

	var file encryptedSecretFile
	if err := json.Unmarshal(data, &file); err != nil {
		return credentialSecret{}, fmt.Errorf("Parsing encrypted account secret: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(file.Nonce)
	if err != nil {
		return credentialSecret{}, ErrCredentialStoreBroken
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(file.Ciphertext)
	if err != nil {
		return credentialSecret{}, ErrCredentialStoreBroken
	}

	key, err := s.loadKey()
	if err != nil {
		return credentialSecret{}, err
	}
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return credentialSecret{}, fmt.Errorf("Creating account secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return credentialSecret{}, fmt.Errorf("Creating account secret cipher mode: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(credentialID))
	if err != nil {
		return credentialSecret{}, ErrCredentialStoreBroken
	}
	defer zeroBytes(plaintext)

	var secret credentialSecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return credentialSecret{}, fmt.Errorf("Parsing account secret: %w", err)
	}
	return secret, nil
}

func (s fileCredentialStore) Delete(context.Context, string) error {
	for _, path := range []string{s.paths.AccountSecretFile(), s.paths.AccountSecretKeyFile()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s fileCredentialStore) loadKey() ([]byte, error) {
	key, err := os.ReadFile(s.paths.AccountSecretKeyFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCredentialSecretNotFound
		}
		return nil, err
	}
	if len(key) != 32 {
		zeroBytes(key)
		return nil, ErrCredentialStoreBroken
	}
	return key, nil
}

func (s fileCredentialStore) loadOrCreateKey() ([]byte, error) {
	if key, err := s.loadKey(); err == nil {
		return key, nil
	} else if !errors.Is(err, ErrCredentialSecretNotFound) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("Generating account secret key: %w", err)
	}
	if err := writePrivateFile(s.paths.AccountSecretKeyFile(), key); err != nil {
		zeroBytes(key)
		return nil, fmt.Errorf("Writing account secret key: %w", err)
	}
	return key, nil
}

func zeroBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
