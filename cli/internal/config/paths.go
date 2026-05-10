package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	ConfigDir  string
	DataDir    string
	RuntimeDir string
	StateDir   string
}

func (p Paths) ConfigFile() string      { return filepath.Join(p.ConfigDir, "config.toml") }
func (p Paths) VaultFile() string       { return filepath.Join(p.DataDir, "vault.forged") }
func (p Paths) AuthDir() string         { return filepath.Join(p.ConfigDir, "auth") }
func (p Paths) CredentialsFile() string { return filepath.Join(p.AuthDir(), "credentials.json") }
func (p Paths) SyncStateFile() string   { return filepath.Join(p.DataDir, "sync-state.json") }
func (p Paths) SyncDirtyFile() string   { return filepath.Join(p.DataDir, "sync.dirty") }
func (p Paths) LocalUnlockBlobFile() string {
	return filepath.Join(p.AuthDir(), "local-unlock.json")
}
func (p Paths) InstallIDFile() string { return filepath.Join(p.AuthDir(), "device.id") }
func (p Paths) HeadlessUnlockKeyFile() string {
	return filepath.Join(p.AuthDir(), "headless-unlock.key")
}

func (p Paths) SSHManagedDir() string {
	return filepath.Join(p.ConfigDir, "ssh")
}

func (p Paths) SSHUserConfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func (p Paths) SSHManagedConfig() string {
	return filepath.Join(p.SSHManagedDir(), "forged.conf")
}

func (p Paths) SSHManagedKeysDir() string {
	return filepath.Join(p.SSHManagedDir(), "keys")
}

func (p Paths) SSHRouteRuntimeDir() string {
	return filepath.Join(p.RuntimeDir, "ssh-routes")
}

func (p Paths) SSHLegacyAdvancedConfig() string {
	return filepath.Join(p.SSHManagedDir(), "config")
}

func (p Paths) LegacySSHManagedDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "forged")
}

func (p Paths) LegacySSHBaseInclude() string {
	return filepath.Join(p.LegacySSHManagedDir(), "base.conf")
}

func (p Paths) AgentSocket() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\forged-agent`
	}
	return filepath.Join(p.RuntimeDir, "agent.sock")
}

func (p Paths) CtlSocket() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\forged-ctl`
	}
	return filepath.Join(p.RuntimeDir, "ctl.sock")
}
func (p Paths) PIDFile() string { return filepath.Join(p.RuntimeDir, "daemon.pid") }
func (p Paths) LogFile() string { return filepath.Join(p.StateDir, "logs", "forged.log") }

func DefaultPaths() Paths {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".config", "forged")

	return Paths{
		ConfigDir:  base,
		DataDir:    filepath.Join(base, "data"),
		RuntimeDir: defaultRuntimeDir(base),
		StateDir:   base,
	}
}

func defaultRuntimeDir(base string) string {
	if runtime.GOOS == "linux" {
		return filepath.Join(envOrDefault("XDG_RUNTIME_DIR", filepath.Join("/run", "user", uidStr())), "forged")
	}
	return filepath.Join(base, "runtime")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func uidStr() string {
	if runtime.GOOS == "windows" {
		return "0"
	}
	return fmt.Sprintf("%d", os.Getuid())
}
