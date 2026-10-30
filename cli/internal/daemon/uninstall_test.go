package daemon

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/itzzritik/forged/cli/internal/config"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
)

func TestRemoveForgedFilesNeverQueuesTheKeptVault(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user cannot read")
	}
	root := t.TempDir()
	paths := config.Paths{ConfigDir: filepath.Join(root, "forged"), DataDir: filepath.Join(root, "forged", "data")}
	if err := os.MkdirAll(paths.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.VaultFile(), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(paths.ConfigDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(paths.ConfigDir, 0o700) })

	for _, path := range removeForgedFiles(paths, nil, true) {
		if inDir(path, paths.VaultFile()) {
			t.Fatalf("%s holds the kept vault but is queued for deletion after exit", path)
		}
	}
}

func TestRemoveForgedFilesKeepsOnlyVault(t *testing.T) {
	setup := func(t *testing.T) (config.Paths, string) {
		root := t.TempDir()
		paths := config.Paths{ConfigDir: filepath.Join(root, "forged"), DataDir: filepath.Join(root, "forged", "data")}
		extra := filepath.Join(root, "install")
		files := []string{
			filepath.Join(paths.ConfigDir, "config.toml"),
			filepath.Join(paths.ConfigDir, "auth", "account.json"),
			paths.VaultFile(),
			paths.VaultFile() + ".pending-wrap",
			paths.SyncStateFile(),
			filepath.Join(extra, "bin", "forged"),
		}
		for _, file := range files {
			if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return paths, extra
	}

	t.Run("keep vault", func(t *testing.T) {
		paths, extra := setup(t)
		if failed := removeForgedFiles(paths, []string{extra}, true); len(failed) != 0 {
			t.Fatalf("failed to remove %q", failed)
		}
		var left []string
		_ = filepath.WalkDir(paths.ConfigDir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				left = append(left, path)
			}
			return nil
		})
		if want := []string{paths.VaultFile(), paths.VaultFile() + ".pending-wrap"}; !slices.Equal(left, want) {
			t.Fatalf("left %q, want only %q", left, want)
		}
		if _, err := os.Stat(extra); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", extra)
		}
	})

	t.Run("delete all", func(t *testing.T) {
		paths, extra := setup(t)
		if failed := removeForgedFiles(paths, []string{extra}, false); len(failed) != 0 {
			t.Fatalf("failed to remove %q", failed)
		}
		for _, dir := range []string{paths.ConfigDir, extra} {
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("%s still exists", dir)
			}
		}
	})
}

func TestVaultProvenOnServer(t *testing.T) {
	base := func() vaultSyncFacts {
		return vaultSyncFacts{
			LoggedIn: true,
			UserID:   "u1",
			State:    &forgedsync.SyncState{LinkedUserID: "u1", LastSyncedHash: "h"},
		}
	}
	if !base().provenOnServer() {
		t.Fatal("a linked, clean, pushed vault must count as on the server")
	}
	cases := map[string]func(*vaultSyncFacts){
		"logged out":         func(f *vaultSyncFacts) { f.LoggedIn = false },
		"no sync state":      func(f *vaultSyncFacts) { f.State = nil },
		"linked to another":  func(f *vaultSyncFacts) { f.State.LinkedUserID = "u2" },
		"dirty state":        func(f *vaultSyncFacts) { f.State.Dirty = true },
		"remote missing":     func(f *vaultSyncFacts) { f.State.RemoteMissing = true },
		"never pushed":       func(f *vaultSyncFacts) { f.State.LastSyncedHash = "" },
		"dirty marker":       func(f *vaultSyncFacts) { f.DirtyMarker = true },
		"password not saved": func(f *vaultSyncFacts) { f.RekeyPending = true },
	}
	for name, change := range cases {
		facts := base()
		change(&facts)
		if facts.provenOnServer() {
			t.Errorf("%s: vault counted as on the server; it must be kept", name)
		}
	}
}
