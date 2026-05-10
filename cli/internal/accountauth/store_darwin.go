//go:build darwin

package accountauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
)

const (
	platformCredentialBackend = "macos_keychain"
	darwinCredentialService   = "com.getforged.account"
)

type darwinCredentialStore struct{}

func newPlatformCredentialStore(config.Paths) credentialStore {
	return darwinCredentialStore{}
}

func (s darwinCredentialStore) Backend() string { return platformCredentialBackend }

func (s darwinCredentialStore) Available(context.Context) bool {
	_, err := exec.LookPath("security")
	return err == nil
}

func (s darwinCredentialStore) Save(ctx context.Context, credentialID string, secret credentialSecret) error {
	body, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	cmd := exec.CommandContext(ctx, "security",
		"add-generic-password",
		"-U",
		"-s", darwinCredentialService,
		"-a", credentialID,
		"-w", encoded,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return darwinCredentialError(out)
	}
	return nil
}

func (s darwinCredentialStore) Load(ctx context.Context, credentialID string) (credentialSecret, error) {
	cmd := exec.CommandContext(ctx, "security",
		"find-generic-password",
		"-s", darwinCredentialService,
		"-a", credentialID,
		"-w",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return credentialSecret{}, darwinCredentialError(out)
	}

	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return credentialSecret{}, ErrCredentialStoreBroken
	}
	var secret credentialSecret
	if err := json.Unmarshal(body, &secret); err != nil {
		return credentialSecret{}, err
	}
	return secret, nil
}

func (s darwinCredentialStore) Delete(ctx context.Context, credentialID string) error {
	cmd := exec.CommandContext(ctx, "security",
		"delete-generic-password",
		"-s", darwinCredentialService,
		"-a", credentialID,
	)
	out, err := cmd.CombinedOutput()
	if err == nil || strings.Contains(string(out), "could not be found") {
		return nil
	}
	return darwinCredentialError(out)
}

func darwinCredentialError(out []byte) error {
	message := string(out)
	switch {
	case strings.Contains(message, "could not be found"):
		return ErrCredentialSecretNotFound
	case strings.Contains(message, "User interaction is not allowed"):
		return ErrCredentialStoreUnavailable
	default:
		return ErrCredentialStoreBroken
	}
}
