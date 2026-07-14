//go:build windows

package sensitiveauth

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/itzzritik/forged/cli/internal/config"
	"golang.org/x/sys/windows"
)

const windowsSecureStoreEntropy = "forged/windows-local-unlock/v1\x00"

const maxWindowsDeviceKeyBlobSize = 64 * 1024

type windowsSecureStore struct {
	paths config.Paths
}

func newPlatformSecureStore(paths config.Paths) SecureStore {
	return &windowsSecureStore{paths: paths}
}

func (s *windowsSecureStore) Capability(context.Context) CapabilityState {
	return CapabilityAvailable
}

func (s *windowsSecureStore) SaveDeviceKey(ctx context.Context, installID, slot string, key []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := localUnlockDeviceKeyFile(s.paths, slot)
	if err != nil {
		return err
	}

	entropy, err := windowsDeviceKeyEntropy(installID)
	if err != nil {
		return err
	}
	protected, err := protectWindowsDeviceKey(key, entropy)
	if err != nil {
		return err
	}
	defer zeroSensitiveBytes(protected)

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeWindowsDeviceKey(path, protected); err != nil {
		return ErrSecureStoreBroken
	}
	return nil
}

func (s *windowsSecureStore) LoadDeviceKey(ctx context.Context, installID, slot string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := localUnlockDeviceKeyFile(s.paths, slot)
	if err != nil {
		return nil, err
	}

	entropy, err := windowsDeviceKeyEntropy(installID)
	if err != nil {
		return nil, err
	}
	protected, err := readWindowsDeviceKey(path)
	if err != nil {
		return nil, err
	}
	defer zeroSensitiveBytes(protected)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := unprotectWindowsDeviceKey(protected, entropy)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		zeroSensitiveBytes(key)
		return nil, err
	}
	return key, nil
}

func (s *windowsSecureStore) DeleteDeviceKey(ctx context.Context, _ string, slot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := localUnlockDeviceKeyFile(s.paths, slot)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return ErrSecureStoreBroken
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func localUnlockDeviceKeyFile(paths config.Paths, slot string) (string, error) {
	if slot == "" {
		return paths.LocalUnlockDeviceKeyFile(), nil
	}
	if slot != localEnrollmentDeviceKeySlotA && slot != localEnrollmentDeviceKeySlotB {
		return "", ErrSecureStoreBroken
	}
	return paths.LocalUnlockDeviceKeySlotFile(slot), nil
}

func windowsDeviceKeyEntropy(installID string) ([]byte, error) {
	if strings.TrimSpace(installID) == "" {
		return nil, ErrSecureStoreBroken
	}
	return []byte(windowsSecureStoreEntropy + installID), nil
}

func protectWindowsDeviceKey(key, entropy []byte) ([]byte, error) {
	input, err := windowsDataBlob(key)
	if err != nil {
		return nil, err
	}
	entropyBlob, err := windowsDataBlob(entropy)
	if err != nil {
		return nil, err
	}

	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, &entropyBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, ErrSecureStoreBroken
	}
	runtime.KeepAlive(key)
	runtime.KeepAlive(entropy)
	defer freeWindowsDataBlob(&output)
	return copyWindowsDataBlob(output)
}

func unprotectWindowsDeviceKey(protected, entropy []byte) ([]byte, error) {
	input, err := windowsDataBlob(protected)
	if err != nil {
		return nil, err
	}
	entropyBlob, err := windowsDataBlob(entropy)
	if err != nil {
		return nil, err
	}

	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, &entropyBlob, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, ErrSecureStoreBroken
	}
	runtime.KeepAlive(protected)
	runtime.KeepAlive(entropy)
	defer freeWindowsDataBlob(&output)
	return copyWindowsDataBlob(output)
}

func windowsDataBlob(data []byte) (windows.DataBlob, error) {
	if len(data) == 0 || uint64(len(data)) > uint64(^uint32(0)) {
		return windows.DataBlob{}, ErrSecureStoreBroken
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}, nil
}

func copyWindowsDataBlob(blob windows.DataBlob) ([]byte, error) {
	if blob.Data == nil || blob.Size == 0 || blob.Size > maxWindowsDeviceKeyBlobSize {
		return nil, ErrSecureStoreBroken
	}
	return append([]byte(nil), unsafe.Slice(blob.Data, int(blob.Size))...), nil
}

func freeWindowsDataBlob(blob *windows.DataBlob) {
	if blob.Data == nil {
		return
	}
	if blob.Size > 0 && blob.Size <= maxWindowsDeviceKeyBlobSize {
		zeroSensitiveBytes(unsafe.Slice(blob.Data, int(blob.Size)))
	}
	_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data)))
	blob.Data = nil
	blob.Size = 0
}

func writeWindowsDeviceKey(path string, data []byte) error {
	if len(data) == 0 || len(data) > maxWindowsDeviceKeyBlobSize {
		return ErrSecureStoreBroken
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".local-unlock.dpapi-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceLocalEnrollmentFile(tmpPath, path)
}

func readWindowsDeviceKey(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSecureStoreNotFound
		}
		return nil, ErrSecureStoreBroken
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxWindowsDeviceKeyBlobSize+1))
	if err != nil {
		return nil, ErrSecureStoreBroken
	}
	if len(data) == 0 || len(data) > maxWindowsDeviceKeyBlobSize {
		zeroSensitiveBytes(data)
		return nil, ErrSecureStoreBroken
	}
	return data, nil
}
