package sshrouting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type Manager struct {
	paths    config.Paths
	keyStore *vault.KeyStore
	selfPath string
}

func NewManager(paths config.Paths, keyStore *vault.KeyStore, selfPath string) *Manager {
	return &Manager{
		paths:    paths,
		keyStore: keyStore,
		selfPath: selfPath,
	}
}

func (m *Manager) Refresh() error {
	if err := os.MkdirAll(m.paths.SSHManagedDir(), 0o700); err != nil {
		return err
	}
	_ = os.Remove(m.paths.SSHLegacyAdvancedConfig())

	keys := m.keyStore.List()
	idToRef, routedKeys := BuildRouteKeyRefs(keys)
	validKeyRefs := make(map[string]struct{}, len(routedKeys))
	for _, key := range routedKeys {
		validKeyRefs[key.Ref] = struct{}{}
	}

	state, err := LoadState(m.paths.SSHRoutingStateFile())
	if err != nil {
		return err
	}
	state.migrateRefs(idToRef)
	state.prune(validKeyRefs)
	if err := SaveState(m.paths.SSHRoutingStateFile(), state); err != nil {
		return err
	}

	if err := os.MkdirAll(m.paths.SSHManagedKeysDir(), 0o700); err != nil {
		return err
	}
	if err := syncHintFiles(m.paths.SSHManagedKeysDir(), routedKeys); err != nil {
		return err
	}

	routes := renderRouteBlocks(m.paths, m.selfPath, routedKeys)
	content := config.RenderManagedSSHConfig(m.paths, routes)
	return os.WriteFile(m.paths.SSHManagedConfig(), []byte(content), 0o600)
}

func syncHintFiles(dir string, keys []RouteKeyRef) error {
	valid := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		path := HintFilePath(dir, key.Ref)
		valid[path] = struct{}{}
		if err := os.WriteFile(path, []byte(strings.TrimSpace(key.PublicKey)+"\n"), 0o600); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if _, ok := valid[path]; ok {
			continue
		}
		_ = os.RemoveAll(path)
	}
	return nil
}

func renderRouteBlocks(paths config.Paths, selfPath string, keys []RouteKeyRef) string {
	if len(keys) < 2 {
		return ""
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].Ref < keys[j].Ref
	})

	lines := make([]string, 0, len(keys)*5)
	for _, key := range keys {
		command := fmt.Sprintf("\"%s\" __ssh-match --key %s", selfPath, key.Ref)
		lines = append(lines,
			fmt.Sprintf("Match host github.com exec %s", strconv.Quote(command)),
			"    User git",
			"    IdentitiesOnly yes",
			fmt.Sprintf("    IdentityFile %q", HintFilePath(paths.SSHManagedKeysDir(), key.Ref)),
			"",
		)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
