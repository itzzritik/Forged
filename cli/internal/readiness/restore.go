package readiness

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/config"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

var (
	errInvalidRestorePassword = errors.New("Invalid restore password")
	errNoRemoteLinkedVault    = errors.New("No remote linked vault")
	errRestoreTargetExists    = errors.New("a local vault was created while restoring")
)

var (
	ErrInvalidRestorePassword = errInvalidRestorePassword
	ErrNoRemoteLinkedVault    = errNoRemoteLinkedVault
	ErrRestoreTargetExists    = errRestoreTargetExists
)

type linkedCredentials struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
}

type linkedRestorePlan struct {
	creds    linkedCredentials
	deviceID string
	result   forgedsync.PullResult
}

func RestoreLinkedVault(paths config.Paths, password []byte) error {
	plan, err := prepareLinkedRestore(paths)
	if err != nil {
		return err
	}
	return applyLinkedRestore(paths, plan, password)
}

func prepareLinkedRestore(paths config.Paths) (linkedRestorePlan, error) {
	creds, err := loadLinkedCredentials(paths)
	if err != nil {
		return linkedRestorePlan{}, err
	}

	var state *forgedsync.SyncState
	err = accountauth.WithCredentials(paths, func(current accountauth.Credentials) error {
		if !linkedRestoreAccountMatches(current, creds) {
			return fmt.Errorf("linked account changed while preparing restore")
		}
		return vault.WithVaultLock(paths.VaultFile(), func() error {
			if err := restoreTargetAbsent(paths.VaultFile()); err != nil {
				return err
			}
			var err error
			state, _, err = loadLinkedRestoreState(paths, creds, "")
			return err
		})
	})
	if err != nil {
		return linkedRestorePlan{}, err
	}

	client := forgedsync.NewClient(creds.ServerURL, creds.Token, state.DeviceID)
	result, err := client.Pull()
	if errors.Is(err, forgedsync.ErrNoRemoteVault) {
		return linkedRestorePlan{}, errNoRemoteLinkedVault
	}
	if err != nil {
		return linkedRestorePlan{}, fmt.Errorf("Fetching linked vault: %w", err)
	}

	return linkedRestorePlan{
		creds:    creds,
		deviceID: state.DeviceID,
		result:   result,
	}, nil
}

// loadLinkedRestoreState is called while holding the account and vault locks.
func loadLinkedRestoreState(paths config.Paths, creds linkedCredentials, fallbackDeviceID string) (*forgedsync.SyncState, *forgedsync.StateStore, error) {
	stateStore := forgedsync.NewStateStore(paths.SyncStateFile())
	state, err := stateStore.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("Loading sync state: %w", err)
	}
	stagedState, err := forgedsync.NewStateStore(paths.SyncStateFile() + ".account-change").Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading staged sync state: %w", err)
	}
	if stagedState != nil {
		return nil, nil, fmt.Errorf("%w: staged sync state requires recovery", forgedsync.ErrStateRecoveryRequired)
	}
	if fallbackDeviceID == "" {
		fallbackDeviceID = uuid.NewString()
	}
	if state == nil {
		defaultState := forgedsync.DefaultSyncState(fallbackDeviceID)
		state = &defaultState
	}
	if state.DeviceID == "" {
		state.DeviceID = fallbackDeviceID
	}
	if state.LinkedUserID != "" && state.LinkedUserID != creds.UserID {
		return nil, nil, fmt.Errorf("%w: local sync state belongs to another account", forgedsync.ErrStateRecoveryRequired)
	}
	if state.ServerURL != "" && !sameRestoreServer(state.ServerURL, creds.ServerURL) {
		return nil, nil, fmt.Errorf("%w: local sync state belongs to another server", forgedsync.ErrStateRecoveryRequired)
	}
	return state, stateStore, nil
}

func linkedRestoreAccountMatches(current accountauth.Credentials, plan linkedCredentials) bool {
	return current.UserID == plan.UserID && sameRestoreServer(current.ServerURL, plan.ServerURL)
}

func loadLinkedCredentials(paths config.Paths) (linkedCredentials, error) {
	creds, err := accountauth.EnsureFresh(context.Background(), paths)
	if err != nil {
		return linkedCredentials{}, fmt.Errorf("Loading linked account credentials: %w", err)
	}
	if creds.ServerURL == "" || accountauth.CurrentToken(creds) == "" {
		return linkedCredentials{}, fmt.Errorf("Linked account credentials are incomplete")
	}

	return linkedCredentials{
		ServerURL: creds.ServerURL,
		Token:     accountauth.CurrentToken(creds),
		UserID:    creds.UserID,
	}, nil
}

func applyLinkedRestore(paths config.Paths, plan linkedRestorePlan, password []byte) error {
	header, ciphertext, syncKey, err := buildRestoredVault(plan.result, password)
	if err != nil {
		return err
	}
	defer wipeBytes(syncKey)

	raw := vault.MarshalVault(header, ciphertext)
	return accountauth.WithCredentials(paths, func(current accountauth.Credentials) error {
		if !linkedRestoreAccountMatches(current, plan.creds) {
			return fmt.Errorf("linked account changed while restoring")
		}
		return vault.WithVaultLock(paths.VaultFile(), func() error {
			if err := restoreTargetAbsent(paths.VaultFile()); err != nil {
				return err
			}
			state, stateStore, err := loadLinkedRestoreState(paths, plan.creds, plan.deviceID)
			if err != nil {
				return err
			}
			if err := forgedsync.ValidateStateHistoryWithKey(syncKey, state); err != nil {
				if errors.Is(err, forgedsync.ErrStateCorrupt) {
					if quarantineErr := stateStore.Quarantine(); quarantineErr != nil {
						return fmt.Errorf("quarantining sync state: %w", quarantineErr)
					}
				}
				return fmt.Errorf("validating sync state: %w", err)
			}
			if err := writeAtomicFile(paths.VaultFile(), raw); err != nil {
				return fmt.Errorf("Writing restored vault: %w", err)
			}

			state.LinkedUserID = plan.creds.UserID
			state.ServerURL = plan.creds.ServerURL
			state.Dirty = false
			state.LastKnownServerVersion = plan.result.Version
			state.LastSyncedBaseBlob = append([]byte(nil), plan.result.Blob...)
			state.LastSyncedHash = hashSyncBlob(plan.result.Blob)
			state.LastSuccessfulPullAt = time.Now().UTC()
			state.LastError = ""
			state.NextRetryAt = time.Time{}

			if err := stateStore.Save(state); err != nil {
				return fmt.Errorf("Saving restored sync state: %w", err)
			}
			return nil
		})
	})
}

// restoreTargetAbsent is called while holding vault's persistent writer lock.
func restoreTargetAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return errRestoreTargetExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Checking restore target: %w", err)
	}
	return nil
}

func buildRestoredVault(result forgedsync.PullResult, password []byte) (vault.Header, []byte, []byte, error) {
	if result.KDFParams == nil || result.ProtectedSymmetricKey == nil || *result.ProtectedSymmetricKey == "" {
		return vault.Header{}, nil, nil, fmt.Errorf("remote vault metadata is incomplete")
	}
	if len(result.Blob) < vault.NonceSize {
		return vault.Header{}, nil, nil, fmt.Errorf("remote vault blob is invalid")
	}

	kdf, err := decodeRestoreKDF(result)
	if err != nil {
		return vault.Header{}, nil, nil, err
	}

	protectedKey, err := decodeProtectedRestoreKey(*result.ProtectedSymmetricKey)
	if err != nil {
		return vault.Header{}, nil, nil, err
	}

	masterKey, err := vault.DeriveKey(password, kdf)
	if err != nil {
		return vault.Header{}, nil, nil, fmt.Errorf("invalid remote vault KDF parameters: %w", err)
	}
	defer wipeBytes(masterKey)

	stretchedKey, err := vault.DeriveStretchedKey(masterKey)
	if err != nil {
		return vault.Header{}, nil, nil, fmt.Errorf("deriving stretched restore key: %w", err)
	}
	defer wipeBytes(stretchedKey)

	symmetricKey, err := vault.DecryptCombined(stretchedKey, protectedKey[:])
	if err != nil {
		return vault.Header{}, nil, nil, errInvalidRestorePassword
	}

	plaintext, err := vault.DecryptCombined(symmetricKey, result.Blob)
	if err != nil {
		wipeBytes(symmetricKey)
		return vault.Header{}, nil, nil, fmt.Errorf("decrypting linked vault: %w", err)
	}
	wipeBytes(plaintext)

	var nonce [vault.NonceSize]byte
	copy(nonce[:], result.Blob[:vault.NonceSize])

	return vault.Header{
		Version:      vault.CurrentVersion,
		KDF:          kdf,
		ProtectedKey: protectedKey,
		Nonce:        nonce,
	}, append([]byte(nil), result.Blob[vault.NonceSize:]...), symmetricKey, nil
}

func sameRestoreServer(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

func decodeRestoreKDF(result forgedsync.PullResult) (vault.KDFParams, error) {
	salt, err := base64.StdEncoding.DecodeString(result.KDFParams.Salt)
	if err != nil {
		return vault.KDFParams{}, fmt.Errorf("Decoding remote vault salt: %w", err)
	}
	if len(salt) != vault.SaltSize {
		return vault.KDFParams{}, fmt.Errorf("Remote vault salt has unexpected length %d", len(salt))
	}

	var kdf vault.KDFParams
	copy(kdf.Salt[:], salt)
	kdf.TimeCost = result.KDFParams.Time
	kdf.MemoryCost = result.KDFParams.Memory
	kdf.Parallelism = result.KDFParams.Parallelism
	return kdf, nil
}

func decodeProtectedRestoreKey(encoded string) ([vault.ProtectedKeySize]byte, error) {
	var protectedKey [vault.ProtectedKeySize]byte

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return protectedKey, fmt.Errorf("Decoding protected symmetric key: %w", err)
	}
	if len(raw) != vault.ProtectedKeySize {
		return protectedKey, fmt.Errorf("Protected symmetric key has unexpected length %d", len(raw))
	}

	copy(protectedKey[:], raw)
	return protectedKey, nil
}

func writeAtomicFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".restore-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

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

	return os.Rename(tmp.Name(), path)
}

func hashSyncBlob(blob []byte) string {
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:])
}

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
