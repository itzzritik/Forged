//go:build windows

package accountauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
)

const platformCredentialBackend = "windows_dpapi"

type windowsCredentialStore struct {
	paths config.Paths
}

func newPlatformCredentialStore(paths config.Paths) credentialStore {
	return windowsCredentialStore{paths: paths}
}

func (s windowsCredentialStore) Backend() string { return platformCredentialBackend }

func (s windowsCredentialStore) Available(context.Context) bool {
	return powershellPath() != ""
}

func (s windowsCredentialStore) Save(ctx context.Context, _ string, secret credentialSecret) error {
	body, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	encrypted, err := runDPAPI(ctx, "protect", body)
	if err != nil {
		return err
	}
	if err := writePrivateFile(s.secretPath(), encrypted); err != nil {
		return fmt.Errorf("Writing DPAPI account secret: %w", err)
	}
	return nil
}

func (s windowsCredentialStore) Load(ctx context.Context, _ string) (credentialSecret, error) {
	encrypted, err := os.ReadFile(s.secretPath())
	if err != nil {
		if os.IsNotExist(err) {
			return credentialSecret{}, ErrCredentialSecretNotFound
		}
		return credentialSecret{}, err
	}
	body, err := runDPAPI(ctx, "unprotect", encrypted)
	if err != nil {
		return credentialSecret{}, err
	}
	var secret credentialSecret
	if err := json.Unmarshal(body, &secret); err != nil {
		return credentialSecret{}, err
	}
	return secret, nil
}

func (s windowsCredentialStore) Delete(context.Context, string) error {
	if err := os.Remove(s.secretPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s windowsCredentialStore) secretPath() string {
	return filepath.Join(s.paths.AuthDir(), "account-secret.dpapi")
}

func runDPAPI(ctx context.Context, mode string, input []byte) ([]byte, error) {
	powershell := powershellPath()
	if powershell == "" {
		return nil, ErrCredentialStoreUnavailable
	}

	var script string
	switch mode {
	case "protect":
		script = `$ErrorActionPreference = 'Stop'; $b = [Convert]::FromBase64String($env:FORGED_DPAPI_INPUT); $e = [System.Security.Cryptography.ProtectedData]::Protect($b, $null, [System.Security.Cryptography.DataProtectionScope]::CurrentUser); [Console]::Out.Write([Convert]::ToBase64String($e))`
	case "unprotect":
		script = `$ErrorActionPreference = 'Stop'; $b = [Convert]::FromBase64String($env:FORGED_DPAPI_INPUT); $p = [System.Security.Cryptography.ProtectedData]::Unprotect($b, $null, [System.Security.Cryptography.DataProtectionScope]::CurrentUser); [Console]::Out.Write([Convert]::ToBase64String($p))`
	default:
		return nil, ErrCredentialStoreBroken
	}

	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "FORGED_DPAPI_INPUT="+base64.StdEncoding.EncodeToString(input))
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(out))
		if strings.Contains(message, "protecteddata") || strings.Contains(message, "denied") {
			return nil, ErrCredentialStoreUnavailable
		}
		return nil, ErrCredentialStoreBroken
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, ErrCredentialStoreBroken
	}
	return decoded, nil
}

func powershellPath() string {
	for _, name := range []string{"powershell.exe", "powershell", "pwsh.exe", "pwsh"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
