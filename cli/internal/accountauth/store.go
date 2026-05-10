package accountauth

import (
	"context"
	"errors"

	"github.com/itzzritik/forged/cli/internal/config"
)

const (
	backendEncryptedFile = "encrypted_file"
)

var (
	ErrCredentialStoreUnavailable = errors.New("credential store unavailable")
	ErrCredentialSecretNotFound   = errors.New("credential secret not found")
	ErrCredentialStoreBroken      = errors.New("credential store broken")
)

type credentialStore interface {
	Backend() string
	Available(context.Context) bool
	Save(context.Context, string, credentialSecret) error
	Load(context.Context, string) (credentialSecret, error)
	Delete(context.Context, string) error
}

func preferredCredentialStore(ctx context.Context, paths config.Paths) credentialStore {
	store := newPlatformCredentialStore(paths)
	if store != nil && store.Available(ctx) {
		return store
	}
	return newFileCredentialStore(paths)
}

func storeForBackend(paths config.Paths, backend string) credentialStore {
	if backend == "" || backend == backendEncryptedFile {
		return newFileCredentialStore(paths)
	}
	if backend == platformCredentialBackend {
		return newPlatformCredentialStore(paths)
	}
	return unsupportedCredentialStore{backend: backend}
}

type unsupportedCredentialStore struct {
	backend string
}

func (s unsupportedCredentialStore) Backend() string { return s.backend }
func (s unsupportedCredentialStore) Available(context.Context) bool {
	return false
}
func (s unsupportedCredentialStore) Save(context.Context, string, credentialSecret) error {
	return ErrCredentialStoreUnavailable
}
func (s unsupportedCredentialStore) Load(context.Context, string) (credentialSecret, error) {
	return credentialSecret{}, ErrCredentialStoreUnavailable
}
func (s unsupportedCredentialStore) Delete(context.Context, string) error {
	return nil
}
