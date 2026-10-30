//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveFromPathRestoresRCFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("SHELL", "/bin/zsh")
	if err := os.MkdirAll(filepath.Join(home, ".config", "fish"), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := map[string]string{
		filepath.Join(home, ".zshrc"):  "export FOO=1\n",
		filepath.Join(home, ".bashrc"): "alias ll='ls -l'\n",
	}
	for file, content := range seed {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if changed, err := EnsureOnPath(); err != nil || !changed {
		t.Fatalf("EnsureOnPath() = %v, %v; want a change", changed, err)
	}

	if err := RemoveFromPath(); err != nil {
		t.Fatal(err)
	}

	for file, want := range seed {
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", file, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "fish", "conf.d", "forged.fish")); !os.IsNotExist(err) {
		t.Error("forged.fish still exists")
	}
}
