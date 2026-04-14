package sshrouting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type RoutingState struct {
	Routes map[string]string `json:"routes,omitempty"`
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
	var payload struct {
		Routes     map[string]string `json:"routes,omitempty"`
		RepoRoutes map[string]string `json:"repo_routes,omitempty"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return newState(), nil
	}
	state.Routes = payload.Routes
	if len(state.Routes) == 0 && len(payload.RepoRoutes) > 0 {
		state.Routes = payload.RepoRoutes
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
	if s.Routes == nil {
		s.Routes = map[string]string{}
	}
}

func (s *RoutingState) prune(validKeyIDs map[string]struct{}) {
	for routeKey, keyID := range s.Routes {
		if _, ok := validKeyIDs[keyID]; !ok {
			delete(s.Routes, routeKey)
		}
	}
}

func (s *RoutingState) migrateRefs(idToRef map[string]string) {
	for routeKey, value := range s.Routes {
		if ref, ok := idToRef[value]; ok {
			s.Routes[routeKey] = ref
		}
	}
}

func newState() RoutingState {
	return RoutingState{
		Routes: map[string]string{},
	}
}
