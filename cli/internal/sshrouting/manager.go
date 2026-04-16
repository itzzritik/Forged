package sshrouting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
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

func (m *Manager) Refresh() error {
	if err := os.MkdirAll(m.paths.SSHManagedDir(), 0o700); err != nil {
		return err
	}

	_ = os.Remove(m.paths.SSHLegacyAdvancedConfig())
	_ = os.Remove(filepath.Join(m.paths.SSHManagedDir(), "routing.json"))
	_ = os.RemoveAll(filepath.Join(m.paths.SSHManagedDir(), "keys"))

	routes := renderRouteHooks(m.selfPath)
	content := config.RenderManagedSSHConfig(m.paths, routes)
	return os.WriteFile(m.paths.SSHManagedConfig(), []byte(content), 0o600)
}

func renderRouteHooks(selfPath string) string {
	prepare := fmt.Sprintf("\"%s\" __ssh-route-prepare --attempt %%C --host %%h --port %%p --user %%r", selfPath)
	success := fmt.Sprintf("\"%s\" __ssh-route-success --attempt %%C --host %%h --port %%p --user %%r", selfPath)
	return strings.Join([]string{
		fmt.Sprintf("Match exec %q", prepare),
		fmt.Sprintf("    LocalCommand %q", success),
	}, "\n")
}
