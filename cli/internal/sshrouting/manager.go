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

	keys := m.keyStore.List()
	validKeyIDs := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		validKeyIDs[key.ID] = struct{}{}
	}

	state, err := LoadState(m.paths.SSHRoutingStateFile())
	if err != nil {
		return err
	}
	state.prune(validKeyIDs)
	if err := SaveState(m.paths.SSHRoutingStateFile(), state); err != nil {
		return err
	}

	if err := os.MkdirAll(m.paths.SSHManagedKeysDir(), 0o700); err != nil {
		return err
	}
	if err := syncHintFiles(m.paths.SSHManagedKeysDir(), keys); err != nil {
		return err
	}

	content := renderAdvancedConfig(m.paths, m.selfPath, keys)
	return os.WriteFile(m.paths.SSHAdvancedConfig(), []byte(content), 0o600)
}

func syncHintFiles(dir string, keys []vault.Key) error {
	valid := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		path := HintFilePath(dir, key.ID)
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

func renderAdvancedConfig(paths config.Paths, selfPath string, keys []vault.Key) string {
	if len(keys) < 2 {
		return "# Managed by Forged\n"
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].CreatedAt.Before(keys[j].CreatedAt)
	})

	lines := []string{"# Managed by Forged"}
	for _, key := range keys {
		command := fmt.Sprintf("\"%s\" __ssh-route-match --key-id %s", selfPath, key.ID)
		lines = append(lines,
			fmt.Sprintf("Match host github.com exec %s", strconv.Quote(command)),
			"    User git",
			"    IdentitiesOnly yes",
			fmt.Sprintf("    IdentityFile %q", HintFilePath(paths.SSHManagedKeysDir(), key.ID)),
			"",
		)
	}
	return strings.Join(lines, "\n")
}
