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
	Security SecurityConfig `toml:"security"`
}

type AgentConfig struct {
	Disabled bool `toml:"disabled"`
}

type SecurityConfig struct {
	MasterPasswordInterval string `toml:"master_password_interval"`
	HeadlessUnlock         bool   `toml:"headless_unlock"`
	HeadlessOffered        bool   `toml:"headless_offered"`
}

const (
	MasterPasswordInterval7Days  = "7d"
	MasterPasswordInterval15Days = "15d"
	MasterPasswordInterval30Days = "30d"
)

func Load(path string) (Config, error) {
	var cfg Config
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
		return saveConfigLocked(path, cfg)
	})
}

func saveConfigLocked(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("Creating config directory: %w", err)
	}

	normalizeConfig(&cfg)

	var body strings.Builder
	body.WriteString("[agent]\n")
	body.WriteString(fmt.Sprintf("disabled = %t\n", cfg.Agent.Disabled))
	body.WriteString("\n[security]\n")
	body.WriteString(fmt.Sprintf("master_password_interval = %q\n", cfg.Security.MasterPasswordInterval))
	body.WriteString(fmt.Sprintf("headless_unlock = %t\n", cfg.Security.HeadlessUnlock))
	body.WriteString(fmt.Sprintf("headless_offered = %t\n", cfg.Security.HeadlessOffered))

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
	return cfg.Agent.Disabled || hasDisabledForgedInclude(paths)
}

func SetAgentDisabled(paths Paths, disabled bool) error {
	return updateConfig(paths, func(cfg *Config) {
		cfg.Agent.Disabled = disabled
	})
}

func SetMasterPasswordInterval(paths Paths, interval string) error {
	return updateConfig(paths, func(cfg *Config) {
		cfg.Security.MasterPasswordInterval = NormalizeMasterPasswordInterval(interval)
	})
}

func HeadlessUnlockEnabled(paths Paths) bool {
	cfg, err := Load(paths.ConfigFile())
	return err == nil && cfg.Security.HeadlessUnlock
}

func SetHeadlessUnlock(paths Paths, enabled bool) error {
	return updateConfig(paths, func(cfg *Config) {
		cfg.Security.HeadlessUnlock = enabled
		cfg.Security.HeadlessOffered = cfg.Security.HeadlessOffered || enabled
	})
}

func SetHeadlessOffered(paths Paths) error {
	return updateConfig(paths, func(cfg *Config) {
		cfg.Security.HeadlessOffered = true
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
		return saveConfigLocked(path, Config{
			Agent: AgentConfig{Disabled: hasDisabledForgedInclude(paths)},
		})
	})
}

func updateConfig(paths Paths, update func(*Config)) error {
	path := paths.ConfigFile()
	return withConfigLock(path, func() error {
		if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Inspecting config: %w", err)
		}
		cfg, err := Load(path)
		if err != nil {
			return err
		}
		update(&cfg)
		return saveConfigLocked(path, cfg)
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
