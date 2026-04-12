package sync

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
	LastSuccessfulPullAt   time.Time `json:"last_successful_pull_at"`
	LastSuccessfulPushAt   time.Time `json:"last_successful_push_at"`
	LastError              string    `json:"last_error"`
	NextRetryAt            time.Time `json:"next_retry_at"`
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
	s.Dirty = false
	s.LastKnownServerVersion = version
	s.LastSyncedBaseBlob = append([]byte(nil), baseBlob...)
	s.LastSyncedHash = hash
	s.LastSuccessfulPushAt = time.Now().UTC()
	s.LastError = ""
	s.NextRetryAt = time.Time{}
}

func (s *StateStore) Load() (*SyncState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.LastSyncedBaseBlobB64 != "" {
		state.LastSyncedBaseBlob, err = base64.StdEncoding.DecodeString(state.LastSyncedBaseBlobB64)
		if err != nil {
			return nil, err
		}
	}

	return &state, nil
}

func (s *StateStore) Save(state *SyncState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	copyState := *state
	if len(copyState.LastSyncedBaseBlob) > 0 {
		copyState.LastSyncedBaseBlobB64 = base64.StdEncoding.EncodeToString(copyState.LastSyncedBaseBlob)
	}

	data, err := json.MarshalIndent(copyState, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o600)
}
