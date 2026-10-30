package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/accountauth"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type vaultSyncFacts struct {
	LoggedIn     bool
	UserID       string
	State        *forgedsync.SyncState
	DirtyMarker  bool
	RekeyPending bool
}

// provenOnServer is the only case where uninstall may delete the vault: anything
// less could lose a key that exists nowhere else.
func (f vaultSyncFacts) provenOnServer() bool {
	s := f.State
	return f.LoggedIn && s != nil && !f.DirtyMarker && !f.RekeyPending &&
		strings.TrimSpace(f.UserID) != "" && s.LinkedUserID == f.UserID &&
		!s.Dirty && !s.RemoteMissing && s.LastSyncedHash != ""
}

func (d *Daemon) vaultSyncFacts() vaultSyncFacts {
	facts := vaultSyncFacts{RekeyPending: vault.RekeyPending(d.paths.VaultFile())}
	if _, err := os.Stat(d.paths.SyncDirtyFile()); err == nil {
		facts.DirtyMarker = true
	}
	if creds, err := accountauth.Load(d.paths); err == nil && (accountauth.CurrentToken(creds) != "" || strings.TrimSpace(creds.RefreshToken) != "") {
		facts.LoggedIn, facts.UserID = true, creds.UserID
	}
	if state, err := forgedsync.NewStateStore(d.paths.SyncStateFile()).Load(); err == nil {
		facts.State = state
	}
	return facts
}

// uninstall removes forged once npm has removed it: the account and this device
// first, then every config line and file, and the service last, because
// stopping the service ends this process.
func (d *Daemon) uninstall(executable string) {
	defer LockRepair(d.paths)()

	facts := d.vaultSyncFacts()
	if facts.LoggedIn {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		creds, err := accountauth.EnsureFresh(ctx, d.paths)
		if err == nil && facts.State != nil {
			err = accountauth.DeleteRemoteDevice(ctx, creds, facts.State.DeviceID)
		}
		cancel()
		if err != nil {
			d.logger.Warn("removing this device from the account failed", "error", err)
		}
		if _, err := d.handleAccountClear(); err != nil {
			d.logger.Warn("logging out during uninstall failed", "error", err)
		}
	}
	// Credentials that did not load (a keychain read error) can still be in the OS store.
	if err := accountauth.Delete(d.paths); err != nil {
		d.logger.Warn("deleting account credentials failed", "error", err)
	}
	d.ClearActiveSession("uninstall")

	steps := []struct {
		name string
		run  func() error
	}{
		{"ssh config", func() error { return config.RemoveSSHIntegration(d.paths) }},
		{"git signing", func() error { return config.RemoveGitSigning(d.paths) }},
		{"git ssh command", config.RemoveGitNativeSSH},
		{"path", RemoveFromPath},
		{"local unlock", func() error { return sensitiveauth.InvalidateLocalEnrollment(d.paths) }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			d.logger.Warn("uninstall step failed", "step", step.name, "error", err)
		}
	}

	dirs := []string{d.paths.RuntimeDir, d.paths.LegacySSHManagedDir(), InstallDir(), LauncherCacheDir()}
	// The daemon's environment can name a different InstallDir than the CLI's; also remove the one it runs from.
	if runtime.GOOS != "windows" && recordedSource(executable) != "" {
		dirs = append(dirs, filepath.Dir(filepath.Dir(executable)))
	}
	keepVault := !facts.provenOnServer()
	failed := removeForgedFiles(d.paths, dirs, keepVault)
	if err := deleteAfterExit(os.Getpid(), failed); err != nil {
		d.logger.Warn("scheduling leftover cleanup failed", "error", err, "paths", failed)
	}
	d.logger.Info("forged uninstalled", "vault_kept", keepVault)
	if err := uninstallServiceSelf(); err != nil {
		d.logger.Warn("removing the service failed", "error", err)
	}
	d.Stop()
}

// removeForgedFiles deletes forged's state and dirs, keeping only the vault and
// its side files when keepVault. It returns what could not be deleted.
func removeForgedFiles(paths config.Paths, dirs []string, keepVault bool) []string {
	var failed []string
	vaultFile := paths.VaultFile()
	var prune func(dir string)
	prune = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				failed = append(failed, dir)
			}
			return
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			switch {
			case filepath.Dir(path) == filepath.Dir(vaultFile) && (path == vaultFile || strings.HasPrefix(path, vaultFile+".")):
			case entry.IsDir() && inDir(path, vaultFile):
				prune(path)
			default:
				if err := os.RemoveAll(path); err != nil {
					failed = append(failed, path)
				}
			}
		}
	}
	for _, dir := range append([]string{paths.ConfigDir, paths.DataDir, paths.StateDir}, dirs...) {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if keepVault && inDir(dir, vaultFile) {
			prune(dir)
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			failed = append(failed, dir)
		}
	}
	// Whatever is returned gets deleted wholesale after exit; a kept vault must not be inside it.
	if keepVault {
		failed = slices.DeleteFunc(failed, func(path string) bool { return inDir(path, vaultFile) })
	}
	return failed
}
