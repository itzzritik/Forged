//go:build !darwin && !linux && !windows

package accountauth

import (
	"context"

	"github.com/itzzritik/forged/cli/internal/config"
)

const platformCredentialBackend = ""

func newPlatformCredentialStore(config.Paths) credentialStore {
	return nil
}

type unavailableCredentialStore struct{}

func (s unavailableCredentialStore) Backend() string { return "" }
func (s unavailableCredentialStore) Available(context.Context) bool {
	return false
}
func (s unavailableCredentialStore) Save(context.Context, string, credentialSecret) error {
	return ErrCredentialStoreUnavailable
}
func (s unavailableCredentialStore) Load(context.Context, string) (credentialSecret, error) {
	return credentialSecret{}, ErrCredentialStoreUnavailable
}
func (s unavailableCredentialStore) Delete(context.Context, string) error {
	return nil
}
