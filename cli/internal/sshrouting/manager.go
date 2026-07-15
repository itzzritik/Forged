package sshrouting

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
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
	if !platform.SSHRoutingSupported() {
		if err := config.EnsureAgentOnlyManagedSSHConfig(m.paths); err != nil {
			return err
		}
		_ = os.RemoveAll(m.paths.SSHRouteRuntimeDir())
		_ = os.RemoveAll(m.paths.SSHManagedKeysDir())
		return nil
	}

	if err := os.MkdirAll(m.paths.SSHManagedDir(), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(m.paths.SSHRouteRuntimeDir(), 0o700); err != nil {
		return err
	}

	refs, err := BuildKeyRefs(keys, m.paths.SSHManagedKeysDir())
	if err != nil {
		return fmt.Errorf("Building SSH key refs: %w", err)
	}
	if err := SyncPublicHintFiles(m.paths.SSHManagedKeysDir(), refs, time.Now().UTC()); err != nil {
		return fmt.Errorf("Syncing SSH public key hints: %w", err)
	}
	routes := renderRouteHooks(m.paths, m.selfPath)
	content := config.RenderManagedSSHConfig(m.paths, routes)
	return config.WriteManagedSSHConfig(m.paths, content)
}

func renderRouteHooks(paths config.Paths, selfPath string) string {
	prepare := strings.Join([]string{
		"exec",
		shellQuote(selfPath),
		"__ssh-route-prepare",
		"--attempt", shellQuote("%C"),
		"--host", shellQuote("%h"),
		"--port", shellQuote("%p"),
		"--user", shellQuote("%r"),
		"--original-host", shellQuote("%n"),
	}, " ")
	success := strings.Join([]string{
		"exec",
		shellQuote(selfPath),
		"__ssh-route-success",
		"--attempt", shellQuote("%C"),
	}, " ")
	return strings.Join([]string{
		fmt.Sprintf("Match exec %s", sshConfigQuote(prepare)),
		fmt.Sprintf("    LocalCommand %s", success),
		renderRouteIdentitySlotHooks(paths, selfPath),
	}, "\n")
}

func renderRouteIdentitySlotHooks(paths config.Paths, selfPath string) string {
	lines := make([]string, 0, routeIdentitySlotCount*2)
	for slot := 1; slot <= routeIdentitySlotCount; slot++ {
		path := routeIdentitySlotPattern(paths.SSHRouteRuntimeDir(), slot)
		check := strings.Join([]string{
			"exec",
			shellQuote(selfPath),
			"__ssh-route-slot",
			"--attempt", shellQuote("%C"),
			"--slot", strconv.Itoa(slot),
		}, " ")
		lines = append(lines,
			fmt.Sprintf("Match exec %s", sshConfigQuote(check)),
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
