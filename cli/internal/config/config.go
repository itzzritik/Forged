package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/itzzritik/forged/cli/internal/platform"
)

type Config struct {
	Agent    AgentConfig    `toml:"agent"`
	Sync     SyncConfig     `toml:"sync"`
	Security SecurityConfig `toml:"security"`
}

type AgentConfig struct {
	Socket   string `toml:"socket"`
	LogLevel string `toml:"log_level"`
	Disabled bool   `toml:"disabled"`
}

type SyncConfig struct {
	Server   string `toml:"server"`
	Interval string `toml:"interval"`
	Enabled  bool   `toml:"enabled"`
}

type SecurityConfig struct {
	MasterPasswordInterval string `toml:"master_password_interval"`
	HeadlessUnlock         bool   `toml:"headless_unlock"`
}

const (
	MasterPasswordInterval7Days  = "7d"
	MasterPasswordInterval15Days = "15d"
	MasterPasswordInterval30Days = "30d"
)

func Load(path string) (Config, error) {
	var cfg Config
	cfg.Agent.LogLevel = "info"
	cfg.Security.MasterPasswordInterval = MasterPasswordInterval7Days

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("Parsing config %s: %w", path, err)
	}

	normalizeConfig(&cfg)
	return cfg, nil
}

func Save(path string, cfg Config) error {
	return withConfigLock(path, func() error {
		return saveConfigLocked(DefaultPaths(), path, cfg, true)
	})
}

func saveConfigLocked(paths Paths, path string, cfg Config, ensureAgentSocket bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating config directory: %w", err)
	}

	if ensureAgentSocket && (strings.TrimSpace(cfg.Agent.Socket) == "" || isLegacyAgentSocket(cfg.Agent.Socket)) {
		if err := paths.ValidateRuntimePaths(); err != nil {
			return err
		}
		cfg.Agent.Socket = paths.AgentSocket()
	}
	if strings.TrimSpace(cfg.Agent.LogLevel) == "" {
		cfg.Agent.LogLevel = "info"
	}
	normalizeConfig(&cfg)

	var body strings.Builder
	body.WriteString("[agent]\n")
	body.WriteString(fmt.Sprintf("socket = %q\n", cfg.Agent.Socket))
	body.WriteString(fmt.Sprintf("log_level = %q\n", cfg.Agent.LogLevel))
	body.WriteString(fmt.Sprintf("disabled = %t\n", cfg.Agent.Disabled))
	body.WriteString("\n[sync]\n")
	if strings.TrimSpace(cfg.Sync.Server) != "" {
		body.WriteString(fmt.Sprintf("server = %q\n", cfg.Sync.Server))
	}
	if strings.TrimSpace(cfg.Sync.Interval) != "" {
		body.WriteString(fmt.Sprintf("interval = %q\n", cfg.Sync.Interval))
	}
	body.WriteString(fmt.Sprintf("enabled = %t\n", cfg.Sync.Enabled))
	body.WriteString("\n[security]\n")
	body.WriteString(fmt.Sprintf("master_password_interval = %q\n", cfg.Security.MasterPasswordInterval))
	body.WriteString(fmt.Sprintf("headless_unlock = %t\n", cfg.Security.HeadlessUnlock))

	return writePrivateFileAtomic(path, []byte(body.String()))
}

func MasterPasswordIntervalDuration(value string) time.Duration {
	switch NormalizeMasterPasswordInterval(value) {
	case MasterPasswordInterval15Days:
		return 15 * 24 * time.Hour
	case MasterPasswordInterval30Days:
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

func normalizeConfig(cfg *Config) {
	cfg.Security.MasterPasswordInterval = NormalizeMasterPasswordInterval(cfg.Security.MasterPasswordInterval)
}

func NormalizeMasterPasswordInterval(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case MasterPasswordInterval15Days:
		return MasterPasswordInterval15Days
	case MasterPasswordInterval30Days:
		return MasterPasswordInterval30Days
	default:
		return MasterPasswordInterval7Days
	}
}

func IsAgentDisabled(paths Paths) bool {
	cfg, err := Load(paths.ConfigFile())
	if err != nil {
		return false
	}
	return cfg.Agent.Disabled
}

type agentSocketUpdatePolicy uint8

const (
	agentSocketPreserve agentSocketUpdatePolicy = iota
	agentSocketEnsure
	agentSocketEnsureOnCreate
)

func SetAgentDisabled(paths Paths, disabled bool) error {
	policy := agentSocketEnsure
	if disabled {
		policy = agentSocketPreserve
	}
	return updateConfig(paths, policy, func(cfg *Config) {
		cfg.Agent.Disabled = disabled
	})
}

func SetMasterPasswordInterval(paths Paths, interval string) error {
	return updateConfig(paths, agentSocketEnsureOnCreate, func(cfg *Config) {
		cfg.Security.MasterPasswordInterval = NormalizeMasterPasswordInterval(interval)
	})
}

func HeadlessUnlockEnabled(paths Paths) bool {
	cfg, err := Load(paths.ConfigFile())
	return err == nil && cfg.Security.HeadlessUnlock
}

func SetHeadlessUnlock(paths Paths, enabled bool) error {
	return updateConfig(paths, agentSocketEnsureOnCreate, func(cfg *Config) {
		cfg.Security.HeadlessUnlock = enabled
	})
}

func EnsureDefault(paths Paths) error {
	path := paths.ConfigFile()
	return withConfigLock(path, func() error {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("Inspecting config: %w", err)
		}
		return saveConfigLocked(paths, path, Config{}, true)
	})
}

func MigrateLegacyAgentSocket(paths Paths) (bool, error) {
	path := paths.ConfigFile()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("Inspecting config: %w", err)
	}

	var migrated bool
	err := withConfigLock(path, func() error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return fmt.Errorf("Inspecting config: %w", err)
		}
		cfg, err := Load(path)
		if err != nil {
			return err
		}
		if !isLegacyAgentSocket(cfg.Agent.Socket) {
			return nil
		}
		if err := paths.ValidateRuntimePaths(); err != nil {
			return err
		}
		cfg.Agent.Socket = paths.AgentSocket()
		if err := saveConfigLocked(paths, path, cfg, false); err != nil {
			return err
		}
		migrated = true
		return nil
	})
	return migrated, err
}

func updateConfig(paths Paths, socketPolicy agentSocketUpdatePolicy, update func(*Config)) error {
	path := paths.ConfigFile()
	return withConfigLock(path, func() error {
		_, err := os.Stat(path)
		configMissing := os.IsNotExist(err)
		if err != nil && !configMissing {
			return fmt.Errorf("Inspecting config: %w", err)
		}
		cfg, err := Load(path)
		if err != nil {
			return err
		}
		update(&cfg)
		ensureAgentSocket := socketPolicy == agentSocketEnsure ||
			(socketPolicy == agentSocketEnsureOnCreate && configMissing)
		return saveConfigLocked(paths, path, cfg, ensureAgentSocket)
	})
}

func withConfigLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating config directory: %w", err)
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("Opening config lock: %w", err)
	}
	defer lock.Close()
	if err := platform.LockFileWait(lock); err != nil {
		return fmt.Errorf("Locking config: %w", err)
	}
	defer platform.UnlockFile(lock)
	return fn()
}

func writePrivateFileAtomic(path string, data []byte) error {
	return writePrivateFile(path, data, false)
}

func writePrivateFileAtomicDurable(path string, data []byte) error {
	return writePrivateFile(path, data, true)
}

func writePrivateFile(path string, data []byte, durable bool) error {
	target, err := resolveWritePath(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("Creating temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("Setting temporary file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("Writing temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("Syncing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing temporary file: %w", err)
	}
	if err := replacePrivateFile(tmpPath, target, durable); err != nil {
		return fmt.Errorf("Replacing %s: %w", target, err)
	}
	if durable && runtime.GOOS != "windows" {
		dir, err := os.Open(filepath.Dir(target))
		if err != nil {
			return fmt.Errorf("Opening private file directory: %w", err)
		}
		defer dir.Close()
		if err := dir.Sync(); err != nil {
			return fmt.Errorf("Syncing private file directory: %w", err)
		}
	}
	return nil
}

func resolveWritePath(path string) (string, error) {
	for range 32 {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return path, nil
		}
		if err != nil {
			return "", fmt.Errorf("Inspecting %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", fmt.Errorf("Reading symlink %s: %w", path, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
	return "", fmt.Errorf("Too many config symlinks")
}
