package sshrouting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itzzritik/forged/cli/internal/config"
)

func TestRefreshWritesSingleHookBasedManagedConfig(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)

	paths := config.Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}
	manager := NewManager(paths, "/tmp/forged")

	if err := manager.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	data, err := os.ReadFile(paths.SSHManagedConfig())
	if err != nil {
		t.Fatalf("read managed config: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "PermitLocalCommand yes") {
		t.Fatalf("expected PermitLocalCommand in config:\n%s", text)
	}
	if !strings.Contains(text, "__ssh-route-prepare") || !strings.Contains(text, "__ssh-route-success") {
		t.Fatalf("expected hidden route hooks in config:\n%s", text)
	}
	if strings.Contains(text, "IdentityFile") {
		t.Fatalf("did not expect per-key IdentityFile routing:\n%s", text)
	}
}

func TestRefreshRemovesLegacyRoutingArtifacts(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)

	paths := config.Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}
	if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
		t.Fatalf("mkdir managed dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(paths.SSHManagedDir(), "keys"), 0o700); err != nil {
		t.Fatalf("mkdir keys dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.SSHManagedDir(), "routing.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write routing.json: %v", err)
	}

	manager := NewManager(paths, "/tmp/forged")
	if err := manager.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if _, err := os.Stat(filepath.Join(paths.SSHManagedDir(), "routing.json")); !os.IsNotExist(err) {
		t.Fatalf("expected routing.json to be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(paths.SSHManagedDir(), "keys")); !os.IsNotExist(err) {
		t.Fatalf("expected managed key dir to be removed, err=%v", err)
	}
}
