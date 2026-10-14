package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		normalizeConfig(&cfg)
		// Only Forged's own lines change, so comments and keys a newer release added survive.
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
			edited := setTOMLValue(string(data), "agent", "disabled", strconv.FormatBool(cfg.Agent.Disabled))
			edited = setTOMLValue(edited, "security", "master_password_interval", strconv.Quote(cfg.Security.MasterPasswordInterval))
			edited = setTOMLValue(edited, "security", "headless_unlock", strconv.FormatBool(cfg.Security.HeadlessUnlock))
			edited = setTOMLValue(edited, "security", "headless_offered", strconv.FormatBool(cfg.Security.HeadlessOffered))
			var check Config
			if _, err := toml.Decode(edited, &check); err == nil {
				normalizeConfig(&check)
				if check == cfg {
					return writePrivateFileAtomic(path, []byte(edited))
				}
			}
		}
		return saveConfigLocked(path, cfg)
	})
}

// setTOMLValue sets key in [section], adding the key or the section when missing.
func setTOMLValue(content, section, key, value string) string {
	lines := strings.Split(content, "\n")
	inSection, insertAt := false, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if inSection {
				break
			}
			inSection = strings.Join(strings.Fields(trimmed), "") == "["+section+"]"
			if inSection {
				insertAt = i + 1
			}
			continue
		}
		if !inSection || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if name, rest, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(name) == key {
			if current, _, _ := strings.Cut(rest, "#"); strings.TrimSpace(current) == value {
				return content
			}
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
		insertAt = i + 1
	}
	if insertAt < 0 {
		return strings.TrimRight(content, "\n") + "\n\n[" + section + "]\n" + key + " = " + value + "\n"
	}
	lines = append(lines[:insertAt], append([]string{key + " = " + value}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n")
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
	target, err := platform.ResolveWritePath(path)
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
	replace := os.Rename
	if durable {
		replace = platform.ReplaceFile
	}
	if err := replace(tmpPath, target); err != nil {
		return fmt.Errorf("Replacing %s: %w", target, err)
	}
	return nil
}
