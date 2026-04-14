package sshrouting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type RoutingState struct {
	RepoRoutes     map[string]string `json:"repo_routes,omitempty"`
	GitHubAccounts map[string]string `json:"github_accounts,omitempty"`
}

func LoadState(path string) (RoutingState, error) {
	var state RoutingState
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(), nil
		}
		return state, err
	}
	if len(data) == 0 {
		return newState(), nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return newState(), nil
	}
	state.ensure()
	return state, nil
}

func SaveState(path string, state RoutingState) error {
	state.ensure()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp."+time.Now().UTC().Format("20060102150405.000000000"))
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *RoutingState) ensure() {
	if s.RepoRoutes == nil {
		s.RepoRoutes = map[string]string{}
	}
	if s.GitHubAccounts == nil {
		s.GitHubAccounts = map[string]string{}
	}
}

func (s *RoutingState) prune(validKeyIDs map[string]struct{}) {
	for routeKey, keyID := range s.RepoRoutes {
		if _, ok := validKeyIDs[keyID]; !ok {
			delete(s.RepoRoutes, routeKey)
		}
	}
	for keyID := range s.GitHubAccounts {
		if _, ok := validKeyIDs[keyID]; !ok {
			delete(s.GitHubAccounts, keyID)
		}
	}
}

func newState() RoutingState {
	return RoutingState{
		RepoRoutes:     map[string]string{},
		GitHubAccounts: map[string]string{},
	}
}
