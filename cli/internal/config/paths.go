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
func (p Paths) CredentialsFile() string { return filepath.Join(p.ConfigDir, "credentials.json") }
func (p Paths) SyncStateFile() string   { return filepath.Join(p.DataDir, "sync-state.json") }
func (p Paths) SyncDirtyFile() string   { return filepath.Join(p.DataDir, "sync.dirty") }
func (p Paths) SSHRoutingStateFile() string {
	return filepath.Join(p.StateDir, "ssh-routing.json")
}

func (p Paths) SSHManagedDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "forged")
}

func (p Paths) SSHUserConfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func (p Paths) SSHBaseInclude() string {
	return filepath.Join(p.SSHManagedDir(), "base.conf")
}

func (p Paths) SSHAdvancedConfig() string {
	return filepath.Join(p.SSHManagedDir(), "config")
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
	switch runtime.GOOS {
	case "linux":
		return linuxPaths()
	case "windows":
		return windowsPaths()
	default:
		return darwinPaths()
	}
}

func windowsPaths() Paths {
	appData := envOrDefault("APPDATA", filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming"))
	base := filepath.Join(appData, "forged")
	return Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}
}

func darwinPaths() Paths {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".forged")
	return Paths{
		ConfigDir:  base,
		DataDir:    base,
		RuntimeDir: base,
		StateDir:   base,
	}
}

func linuxPaths() Paths {
	home, _ := os.UserHomeDir()

	configDir := envOrDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dataDir := envOrDefault("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	runtimeDir := envOrDefault("XDG_RUNTIME_DIR", filepath.Join("/run", "user", uidStr()))
	stateDir := envOrDefault("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	return Paths{
		ConfigDir:  filepath.Join(configDir, "forged"),
		DataDir:    filepath.Join(dataDir, "forged"),
		RuntimeDir: filepath.Join(runtimeDir, "forged"),
		StateDir:   filepath.Join(stateDir, "forged"),
	}
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
