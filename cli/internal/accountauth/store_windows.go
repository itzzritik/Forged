//go:build windows

package accountauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/config"
	"golang.org/x/sys/windows"
)

const platformCredentialBackend = "windows_dpapi"

const (
	maxWindowsCredentialPlaintextSize = 64 * 1024
	maxWindowsCredentialBlobSize      = maxWindowsCredentialPlaintextSize + 4*1024
)

type windowsCredentialStore struct {
	paths config.Paths
}

func newPlatformCredentialStore(paths config.Paths) credentialStore {
	return windowsCredentialStore{paths: paths}
}

func (s windowsCredentialStore) Backend() string { return platformCredentialBackend }

func (s windowsCredentialStore) Available(context.Context) bool { return true }

func (s windowsCredentialStore) Save(ctx context.Context, credentialID string, secret credentialSecret) error {
	body, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	defer zeroBytes(body)
	encrypted, err := runDPAPI(ctx, "protect", body)
	if err != nil {
		return err
	}
	defer zeroBytes(encrypted)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writePrivateFile(s.secretPath(credentialID), encrypted); err != nil {
		return fmt.Errorf("Writing DPAPI account secret: %w", err)
	}
	return nil
}

func (s windowsCredentialStore) Load(ctx context.Context, credentialID string) (credentialSecret, error) {
	encrypted, err := readWindowsCredentialBlob(s.secretPath(credentialID))
	if err != nil {
		return credentialSecret{}, err
	}
	defer zeroBytes(encrypted)
	body, err := runDPAPI(ctx, "unprotect", encrypted)
	if err != nil {
		return credentialSecret{}, err
	}
	defer zeroBytes(body)
	var secret credentialSecret
	if err := json.Unmarshal(body, &secret); err != nil {
		return credentialSecret{}, err
	}
	return secret, nil
}

func (s windowsCredentialStore) Delete(_ context.Context, credentialID string) error {
	if err := os.Remove(s.secretPath(credentialID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s windowsCredentialStore) secretPath(credentialID string) string {
	return credentialSecretPath(filepath.Join(s.paths.AuthDir(), "account-secret.dpapi"), credentialID)
}

func runDPAPI(ctx context.Context, mode string, input []byte) ([]byte, error) {
	// Use DPAPI directly so plaintext never crosses a child-process boundary.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxInputSize := maxWindowsCredentialPlaintextSize
	maxOutputSize := maxWindowsCredentialBlobSize
	if mode == "unprotect" {
		maxInputSize = maxWindowsCredentialBlobSize
		maxOutputSize = maxWindowsCredentialPlaintextSize
	}
	inputBlob, err := windowsCredentialDataBlob(input, maxInputSize)
	if err != nil {
		return nil, err
	}
	var output windows.DataBlob
	defer freeWindowsCredentialDataBlob(&output)
	switch mode {
	case "protect":
		err = windows.CryptProtectData(&inputBlob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	case "unprotect":
		err = windows.CryptUnprotectData(&inputBlob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	default:
		return nil, ErrCredentialStoreBroken
	}
	runtime.KeepAlive(input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
			errors.Is(err, windows.ERROR_FILE_NOT_FOUND) ||
			errors.Is(err, windows.ERROR_PASSWORD_RESTRICTION) {
			return nil, ErrCredentialStoreUnavailable
		}
		return nil, ErrCredentialStoreBroken
	}
	result, err := copyWindowsCredentialDataBlob(output, maxOutputSize)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		zeroBytes(result)
		return nil, err
	}
	return result, nil
}

func windowsCredentialDataBlob(data []byte, maxSize int) (windows.DataBlob, error) {
	if len(data) == 0 || len(data) > maxSize {
		return windows.DataBlob{}, ErrCredentialStoreBroken
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}, nil
}

func copyWindowsCredentialDataBlob(blob windows.DataBlob, maxSize int) ([]byte, error) {
	if blob.Data == nil || blob.Size == 0 || blob.Size > uint32(maxSize) {
		return nil, ErrCredentialStoreBroken
	}
	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...), nil
}

func freeWindowsCredentialDataBlob(blob *windows.DataBlob) {
	if blob.Data == nil {
		return
	}
	if blob.Size > 0 && blob.Size <= maxWindowsCredentialBlobSize {
		zeroBytes(unsafe.Slice(blob.Data, int(blob.Size)))
	}
	_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data)))
	blob.Data = nil
	blob.Size = 0
}

func readWindowsCredentialBlob(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCredentialSecretNotFound
		}
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(maxWindowsCredentialBlobSize)+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxWindowsCredentialBlobSize {
		zeroBytes(data)
		return nil, ErrCredentialStoreBroken
	}
	return data, nil
}
