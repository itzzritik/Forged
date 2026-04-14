package sshrouting

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type HostAffinity struct {
	KeyID          string    `json:"key_id"`
	LastSuccessAt  time.Time `json:"last_success_at"`
	SuccessCount   int       `json:"success_count"`
	LastFailureAt  time.Time `json:"last_failure_at,omitempty"`
	ManualOverride bool      `json:"manual_override,omitempty"`
}

type State struct {
	Hosts map[string]HostAffinity `json:"hosts"`
}

func DefaultState() State {
	return State{Hosts: map[string]HostAffinity{}}
}

func hostKey(host string, port int) string {
	return host + ":" + strconv.Itoa(port)
}

func (s *State) RecordSuccess(host string, port int, keyID string, now time.Time) {
	if s.Hosts == nil {
		s.Hosts = map[string]HostAffinity{}
	}

	key := hostKey(host, port)
	entry := s.Hosts[key]
	entry.KeyID = keyID
	entry.LastSuccessAt = now.UTC()
	entry.SuccessCount++
	entry.LastFailureAt = time.Time{}
	s.Hosts[key] = entry
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (*State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		state := DefaultState()
		return &state, nil
	}
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Hosts == nil {
		state.Hosts = map[string]HostAffinity{}
	}
	return &state, nil
}

func (s *Store) Save(state *State) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o600)
}
