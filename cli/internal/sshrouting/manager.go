package sshrouting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type Manager struct {
	paths    config.Paths
	selfPath string
}

func NewManager(paths config.Paths, selfPath string) *Manager {
	return &Manager{
		paths:    paths,
		selfPath: selfPath,
	}
}

func (m *Manager) Refresh(keys []vault.Key) error {
	if err := os.MkdirAll(m.paths.SSHManagedDir(), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(m.paths.SSHRouteRuntimeDir(), 0o700); err != nil {
		return err
	}

	_ = os.Remove(m.paths.SSHLegacyAdvancedConfig())
	_ = os.Remove(filepath.Join(m.paths.SSHManagedDir(), "routing.json"))

	refs, err := BuildKeyRefs(keys, m.paths.SSHManagedKeysDir())
	if err != nil {
		return fmt.Errorf("Building SSH key refs: %w", err)
	}
	if err := SyncPublicHintFiles(m.paths.SSHManagedKeysDir(), refs, time.Now().UTC()); err != nil {
		return fmt.Errorf("Syncing SSH public key hints: %w", err)
	}
	if err := CleanupRouteRuntime(m.paths.SSHRouteRuntimeDir(), time.Now().UTC().Add(-routeSnippetTTL)); err != nil {
		return fmt.Errorf("Cleaning SSH route snippets: %w", err)
	}

	routes := renderRouteHooks(m.paths, m.selfPath)
	content := config.RenderManagedSSHConfig(m.paths, routes)
	return os.WriteFile(m.paths.SSHManagedConfig(), []byte(content), 0o600)
}

func renderRouteHooks(paths config.Paths, selfPath string) string {
	prepare := strings.Join([]string{
		shellQuote(selfPath),
		"__ssh-route-prepare",
		"--attempt", "%C",
		"--host", "%h",
		"--port", "%p",
		"--user", "%r",
		"--original-host", "%n",
	}, " ")
	success := strings.Join([]string{
		shellQuote(selfPath),
		"__ssh-route-success",
		"--attempt", "%C",
		"--host", "%h",
		"--port", "%p",
		"--user", "%r",
	}, " ")
	return strings.Join([]string{
		fmt.Sprintf("Match exec %s", sshConfigQuote(prepare)),
		"    IdentitiesOnly yes",
		"    IdentityFile none",
		fmt.Sprintf("    LocalCommand %s", success),
		renderRouteIdentitySlotHooks(paths),
	}, "\n")
}

func renderRouteIdentitySlotHooks(paths config.Paths) string {
	lines := make([]string, 0, routeIdentitySlotCount*2)
	for slot := 1; slot <= routeIdentitySlotCount; slot++ {
		path := routeIdentitySlotPattern(paths.SSHRouteRuntimeDir(), slot)
		test := "test -f " + shellQuote(path)
		lines = append(lines,
			fmt.Sprintf("Match exec %s", sshConfigQuote(test)),
			fmt.Sprintf("    IdentityFile %q", path),
		)
	}
	return strings.Join(lines, "\n")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	safe := true
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '/' || r == '.' || r == '_' || r == '-':
		default:
			safe = false
		}
	}
	if safe {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func sshConfigQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
