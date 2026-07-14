package sync

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

var (
	ErrStateCorrupt          = errors.New("sync state is corrupt")
	ErrStateRecoveryRequired = errors.New("sync state recovery is required")
)

type SyncState struct {
	LinkedUserID           string    `json:"linked_user_id"`
	ServerURL              string    `json:"server_url"`
	DeviceID               string    `json:"device_id"`
	LastKnownServerVersion int64     `json:"last_known_server_version"`
	LastSyncedBaseBlob     []byte    `json:"-"`
	LastSyncedBaseBlobB64  string    `json:"last_synced_base_blob"`
	LastSyncedHash         string    `json:"last_synced_hash"`
	Dirty                  bool      `json:"dirty"`
	LastRemoteCheckAt      time.Time `json:"last_remote_check_at"`
	LastSuccessfulPullAt   time.Time `json:"last_successful_pull_at"`
	LastSuccessfulPushAt   time.Time `json:"last_successful_push_at"`
	LastError              string    `json:"last_error"`
	NextRetryAt            time.Time `json:"next_retry_at"`
	Syncing                bool      `json:"-"`
}

type StateStore struct {
	path string
}

func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

func DefaultSyncState(deviceID string) SyncState {
	return SyncState{DeviceID: deviceID}
}

func (s *SyncState) MarkDirty(err string, nextRetry time.Time) {
	s.Dirty = true
	s.LastError = err
	s.NextRetryAt = nextRetry.UTC()
}

func (s *SyncState) MarkClean(version int64, baseBlob []byte, hash string) {
	now := time.Now().UTC()
	s.Dirty = false
	s.LastKnownServerVersion = version
	s.LastSyncedBaseBlob = append([]byte(nil), baseBlob...)
	s.LastSyncedHash = hash
	s.LastRemoteCheckAt = now
	s.LastSuccessfulPushAt = now
	s.LastError = ""
	s.NextRetryAt = time.Time{}
}

func (s *StateStore) Load() (*SyncState, error) {
	recoveryRequired, err := s.RecoveryRequired()
	if err != nil {
		return nil, err
	}
	if recoveryRequired {
		return nil, ErrStateRecoveryRequired
	}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Reading sync state: %w", err)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, s.quarantineCorruptState(corruptStateError("invalid JSON"))
	}
	if stateFieldsIncomplete(object) {
		return nil, s.quarantineCorruptState(corruptStateError("incomplete state"))
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, s.quarantineCorruptState(corruptStateError("invalid JSON"))
	}
	if state.LastSyncedBaseBlobB64 != "" {
		state.LastSyncedBaseBlob, err = base64.StdEncoding.DecodeString(state.LastSyncedBaseBlobB64)
		if err != nil {
			return nil, s.quarantineCorruptState(corruptStateError("invalid merge base encoding"))
		}
	}
	if err := validateState(&state); err != nil {
		return nil, s.quarantineCorruptState(err)
	}

	return &state, nil
}

func (s *StateStore) RecoveryRequired() (bool, error) {
	_, err := os.Stat(s.recoveryMarkerPath())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("reading sync recovery marker: %w", err)
}

func (s *StateStore) Quarantine() error {
	if err := s.writeRecoveryMarker(); err != nil {
		return err
	}

	if _, err := os.Lstat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("reading corrupt sync state: %w", err)
	}
	if err := os.Link(s.path, s.corruptPath()); errors.Is(err, os.ErrExist) {
		return fmt.Errorf("refusing to overwrite quarantined sync state")
	} else if err != nil {
		return fmt.Errorf("quarantining sync state: %w", err)
	}
	if !stateDirectorySyncSupported() {
		// Keep the corrupt source when the platform cannot durably sync a directory.
		// A crash can then never turn the state into a missing fresh link.
		return nil
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("removing corrupt sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	return nil
}

func (s *StateStore) MoveTo(destination string) error {
	if !stateDirectorySyncSupported() {
		return fmt.Errorf("%w: directory sync is unavailable", ErrStateRecoveryRequired)
	}
	if err := os.Link(s.path, destination); errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: destination sync state exists", ErrStateRecoveryRequired)
	} else if err != nil {
		return fmt.Errorf("linking sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("removing moved sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	return nil
}

func (s *StateStore) quarantineCorruptState(corruptErr error) error {
	if err := s.Quarantine(); err != nil {
		return fmt.Errorf("%w: quarantining failed: %v", corruptErr, err)
	}
	return corruptErr
}

func (s *StateStore) recoveryMarkerPath() string {
	return s.path + ".recovery-required"
}

func (s *StateStore) corruptPath() string {
	return s.path + ".corrupt"
}

func (s *StateStore) writeRecoveryMarker() error {
	marker, err := os.OpenFile(s.recoveryMarkerPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating sync recovery marker: %w", err)
	}
	if _, err := marker.WriteString("sync state recovery required\n"); err != nil {
		_ = marker.Close()
		return fmt.Errorf("writing sync recovery marker: %w", err)
	}
	if err := marker.Sync(); err != nil {
		_ = marker.Close()
		return fmt.Errorf("syncing sync recovery marker: %w", err)
	}
	if err := marker.Close(); err != nil {
		return fmt.Errorf("closing sync recovery marker: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	return nil
}

func syncStateDirectory(path string) error {
	if !stateDirectorySyncSupported() {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func stateDirectorySyncSupported() bool {
	return runtime.GOOS != "windows"
}

func ValidateStateHistory(v *vault.Vault, state *SyncState) error {
	if state == nil {
		return corruptStateError("missing state")
	}
	if len(state.LastSyncedBaseBlob) == 0 {
		return nil
	}
	if v == nil {
		return fmt.Errorf("vault is required to validate sync history")
	}

	plaintext, err := v.DecryptSyncBlob(state.LastSyncedBaseBlob)
	if err != nil {
		return corruptStateError("merge base cannot be decrypted")
	}
	defer clear(plaintext)
	return validateStateBase(plaintext)
}

func ValidateStateHistoryWithKey(key []byte, state *SyncState) error {
	if state == nil {
		return corruptStateError("missing state")
	}
	if len(state.LastSyncedBaseBlob) == 0 {
		return nil
	}

	plaintext, err := vault.DecryptCombined(key, state.LastSyncedBaseBlob)
	if err != nil {
		return corruptStateError("merge base cannot be decrypted")
	}
	defer clear(plaintext)
	return validateStateBase(plaintext)
}

func validateStateBase(plaintext []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(plaintext, &object); err != nil || len(object) == 0 {
		return corruptStateError("merge base is invalid")
	}
	for _, name := range []string{"keys", "metadata", "version_vector", "tombstones", "key_generation"} {
		raw, ok := object[name]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			return corruptStateError("merge base is invalid")
		}
	}
	var base vault.VaultData
	if err := json.Unmarshal(plaintext, &base); err != nil {
		return corruptStateError("merge base is invalid")
	}
	return nil
}

func validateState(state *SyncState) error {
	if state.LastKnownServerVersion < 0 {
		return corruptStateError("negative server version")
	}

	linked := strings.TrimSpace(state.LinkedUserID) != ""
	server := strings.TrimSpace(state.ServerURL) != ""
	if linked != server {
		return corruptStateError("incomplete linked account")
	}
	if strings.TrimSpace(state.DeviceID) == "" && !linked {
		return corruptStateError("missing device ID")
	}

	hasBase := len(state.LastSyncedBaseBlob) > 0
	hasHash := strings.TrimSpace(state.LastSyncedHash) != ""
	if hasBase != hasHash {
		return corruptStateError("incomplete merge history")
	}
	if state.LastKnownServerVersion > 0 && !hasBase {
		return corruptStateError("missing merge history")
	}
	if hasBase {
		if state.LastKnownServerVersion == 0 || !linked {
			return corruptStateError("incoherent merge history")
		}
		sum := sha256.Sum256(state.LastSyncedBaseBlob)
		if !strings.EqualFold(state.LastSyncedHash, hex.EncodeToString(sum[:])) {
			return corruptStateError("merge history hash mismatch")
		}
	}
	return nil
}

func corruptStateError(reason string) error {
	return fmt.Errorf("%w: %s", ErrStateCorrupt, reason)
}

func stateFieldsIncomplete(object map[string]json.RawMessage) bool {
	for _, name := range []string{"linked_user_id", "server_url", "last_known_server_version", "last_synced_base_blob", "last_synced_hash", "dirty"} {
		raw, ok := object[name]
		if !ok || strings.TrimSpace(string(raw)) == "null" {
			return true
		}
	}
	raw, ok := object["device_id"]
	return ok && strings.TrimSpace(string(raw)) == "null"
}

func (s *StateStore) Save(state *SyncState) error {
	recoveryRequired, err := s.RecoveryRequired()
	if err != nil {
		return err
	}
	if recoveryRequired {
		return ErrStateRecoveryRequired
	}
	if state == nil {
		return corruptStateError("missing state")
	}
	if err := validateState(state); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("Creating sync state directory: %w", err)
	}

	copyState := *state
	copyState.LastSyncedBaseBlobB64 = base64.StdEncoding.EncodeToString(copyState.LastSyncedBaseBlob)

	data, err := json.MarshalIndent(copyState, "", "  ")
	if err != nil {
		return fmt.Errorf("Encoding sync state: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".sync-state.tmp-*")
	if err != nil {
		return fmt.Errorf("Creating temporary sync state: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("Securing temporary sync state: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("Writing temporary sync state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("Syncing temporary sync state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing temporary sync state: %w", err)
	}
	if err := replaceSyncStateFile(tmpPath, s.path); err != nil {
		return fmt.Errorf("Replacing sync state: %w", err)
	}
	if err := syncStateDirectory(filepath.Dir(s.path)); err != nil {
		return fmt.Errorf("syncing sync state directory: %w", err)
	}
	return nil
}
