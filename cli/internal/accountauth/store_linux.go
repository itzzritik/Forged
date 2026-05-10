//go:build linux

package accountauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
)

const platformCredentialBackend = "linux_secret_service"

type linuxCredentialStore struct{}

func newPlatformCredentialStore(config.Paths) credentialStore {
	return linuxCredentialStore{}
}

func (s linuxCredentialStore) Backend() string { return platformCredentialBackend }

func (s linuxCredentialStore) Available(context.Context) bool {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return false
	}
	return strings.TrimSpace(os.Getenv("DBUS_SESSION_BUS_ADDRESS")) != ""
}

func (s linuxCredentialStore) Save(ctx context.Context, credentialID string, secret credentialSecret) error {
	body, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	args := append([]string{"store", "--label", "Forged account credentials"}, linuxSecretToolAttributes(credentialID)...)
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	cmd.Stdin = strings.NewReader(encoded + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return linuxCredentialError(out)
	}
	return nil
}

func (s linuxCredentialStore) Load(ctx context.Context, credentialID string) (credentialSecret, error) {
	args := append([]string{"lookup"}, linuxSecretToolAttributes(credentialID)...)
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return credentialSecret{}, linuxCredentialError(out)
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return credentialSecret{}, ErrCredentialSecretNotFound
	}
	body, err := base64.StdEncoding.DecodeString(string(out))
	if err != nil {
		return credentialSecret{}, ErrCredentialStoreBroken
	}
	var secret credentialSecret
	if err := json.Unmarshal(body, &secret); err != nil {
		return credentialSecret{}, err
	}
	return secret, nil
}

func (s linuxCredentialStore) Delete(ctx context.Context, credentialID string) error {
	args := append([]string{"clear"}, linuxSecretToolAttributes(credentialID)...)
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return linuxCredentialError(out)
	}
	return nil
}

func linuxSecretToolAttributes(credentialID string) []string {
	return []string{
		"application", "forged",
		"kind", "account",
		"credential_id", credentialID,
	}
}

func linuxCredentialError(out []byte) error {
	message := strings.ToLower(string(out))
	switch {
	case strings.Contains(message, "no such") || strings.Contains(message, "not found"):
		return ErrCredentialSecretNotFound
	case strings.Contains(message, "cannot autolaunch") ||
		strings.Contains(message, "no medium found") ||
		strings.Contains(message, "not available") ||
		strings.Contains(message, "cannot open display"):
		return ErrCredentialStoreUnavailable
	default:
		return ErrCredentialStoreBroken
	}
}
