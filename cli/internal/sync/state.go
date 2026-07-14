package sync

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Reading sync state: %w", err)
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("Parsing sync state: %w", err)
	}
	if state.LastSyncedBaseBlobB64 != "" {
		state.LastSyncedBaseBlob, err = base64.StdEncoding.DecodeString(state.LastSyncedBaseBlobB64)
		if err != nil {
			return nil, fmt.Errorf("Decoding sync base: %w", err)
		}
	}

	return &state, nil
}

func (s *StateStore) Save(state *SyncState) error {
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
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("Replacing sync state: %w", err)
	}
	return nil
}
