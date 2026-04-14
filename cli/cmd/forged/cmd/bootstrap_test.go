package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itzzritik/forged/cli/internal/config"
)

func TestEnsureDefaultConfigWritesConfigWhenMissing(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	paths := config.Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}

	if err := ensureDefaultConfig(paths); err != nil {
		t.Fatalf("ensureDefaultConfig() error = %v", err)
	}

	raw, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	got := string(raw)
	if !strings.Contains(got, paths.AgentSocket()) {
		t.Fatalf("config did not include agent socket %q: %s", paths.AgentSocket(), got)
	}
	if !strings.Contains(got, `enabled = false`) {
		t.Fatalf("config did not disable sync by default: %s", got)
	}
}

func TestEnsureDefaultConfigDoesNotOverwriteExistingConfig(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	paths := config.Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}

	original := []byte("[agent]\nlog_level = \"debug\"\n")
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o700); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(paths.ConfigFile(), original, 0o600); err != nil {
		t.Fatalf("writing original config: %v", err)
	}

	if err := ensureDefaultConfig(paths); err != nil {
		t.Fatalf("ensureDefaultConfig() error = %v", err)
	}

	raw, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	if string(raw) != string(original) {
		t.Fatalf("config was overwritten:\nwant:\n%s\ngot:\n%s", string(original), string(raw))
	}
}

func TestCreateVaultAtPathsCreatesVaultFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	paths := config.Paths{
		ConfigDir:  filepath.Join(base, "config"),
		DataDir:    filepath.Join(base, "data"),
		RuntimeDir: filepath.Join(base, "runtime"),
		StateDir:   filepath.Join(base, "state"),
	}

	v, ks, err := createVaultAtPaths(paths, []byte("password123"))
	if err != nil {
		t.Fatalf("createVaultAtPaths() error = %v", err)
	}
	defer v.Close()

	if _, err := os.Stat(paths.VaultFile()); err != nil {
		t.Fatalf("vault file missing: %v", err)
	}
	if ks == nil {
		t.Fatalf("expected non-nil keystore")
	}
	if len(ks.List()) != 0 {
		t.Fatalf("expected empty keystore, got %d keys", len(ks.List()))
	}
}
