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

type ProviderKey struct {
	Provider        string    `json:"provider"`
	KeyID           string    `json:"key_id"`
	MatchHost       string    `json:"match_host"`
	Alias           string    `json:"alias"`
	HintPath        string    `json:"hint_path"`
	LastRefreshedAt time.Time `json:"last_refreshed_at,omitempty"`
}

type RepoRoute struct {
	Provider       string    `json:"provider"`
	MatchHost      string    `json:"match_host"`
	RepoKey        string    `json:"repo_key"`
	KeyID          string    `json:"key_id"`
	LastVerifiedAt time.Time `json:"last_verified_at,omitempty"`
	SuccessCount   int       `json:"success_count,omitempty"`
}

type State struct {
	Hosts        map[string]HostAffinity `json:"hosts"`
	ProviderKeys map[string]ProviderKey  `json:"provider_keys,omitempty"`
	RepoRoutes   map[string]RepoRoute    `json:"repo_routes,omitempty"`
}

func DefaultState() State {
	return State{
		Hosts:        map[string]HostAffinity{},
		ProviderKeys: map[string]ProviderKey{},
		RepoRoutes:   map[string]RepoRoute{},
	}
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

func (s *State) UpsertProviderKey(key ProviderKey) {
	if s.ProviderKeys == nil {
		s.ProviderKeys = map[string]ProviderKey{}
	}
	s.ProviderKeys[key.KeyID] = key
}

func (s *State) RemoveMissingProviderKeys(valid map[string]struct{}) {
	if s.ProviderKeys == nil {
		return
	}
	for keyID := range s.ProviderKeys {
		if _, ok := valid[keyID]; !ok {
			delete(s.ProviderKeys, keyID)
		}
	}
	if s.RepoRoutes == nil {
		return
	}
	for repoKey, route := range s.RepoRoutes {
		if _, ok := valid[route.KeyID]; !ok {
			delete(s.RepoRoutes, repoKey)
		}
	}
}

func (s *State) UpsertRepoRoute(route RepoRoute) {
	if s.RepoRoutes == nil {
		s.RepoRoutes = map[string]RepoRoute{}
	}
	existing := s.RepoRoutes[route.RepoKey]
	route.SuccessCount = existing.SuccessCount + 1
	route.LastVerifiedAt = route.LastVerifiedAt.UTC()
	s.RepoRoutes[route.RepoKey] = route
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
	if state.ProviderKeys == nil {
		state.ProviderKeys = map[string]ProviderKey{}
	}
	if state.RepoRoutes == nil {
		state.RepoRoutes = map[string]RepoRoute{}
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
