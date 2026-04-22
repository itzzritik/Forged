package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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
	ExternalUsePolicy      string `toml:"external_use_policy"`
}

const (
	MasterPasswordInterval7Days  = "7d"
	MasterPasswordInterval15Days = "15d"
	MasterPasswordInterval30Days = "30d"

	ExternalUsePolicyDeny  = "deny"
	ExternalUsePolicyAllow = "allow"
)

func Load(path string) (Config, error) {
	var cfg Config
	cfg.Agent.LogLevel = "info"
	cfg.Security.MasterPasswordInterval = MasterPasswordInterval7Days
	cfg.Security.ExternalUsePolicy = ExternalUsePolicyDeny

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}

	normalizeConfig(&cfg)
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	if strings.TrimSpace(cfg.Agent.Socket) == "" {
		cfg.Agent.Socket = DefaultPaths().AgentSocket()
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
	body.WriteString("\n\n[security]\n")
	body.WriteString(fmt.Sprintf("master_password_interval = %q\n", cfg.Security.MasterPasswordInterval))
	body.WriteString(fmt.Sprintf("external_use_policy = %q\n", cfg.Security.ExternalUsePolicy))

	return os.WriteFile(path, []byte(body.String()), 0o600)
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
	if strings.TrimSpace(cfg.Security.ExternalUsePolicy) == "" {
		cfg.Security.ExternalUsePolicy = ExternalUsePolicyDeny
	}
	switch strings.TrimSpace(strings.ToLower(cfg.Security.ExternalUsePolicy)) {
	case ExternalUsePolicyAllow:
		cfg.Security.ExternalUsePolicy = ExternalUsePolicyAllow
	default:
		cfg.Security.ExternalUsePolicy = ExternalUsePolicyDeny
	}
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

func SetAgentDisabled(paths Paths, disabled bool) error {
	cfg, err := Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Agent.Socket) == "" {
		cfg.Agent.Socket = paths.AgentSocket()
	}
	cfg.Agent.Disabled = disabled
	return Save(paths.ConfigFile(), cfg)
}

func SetExternalUsePolicy(paths Paths, policy string) error {
	cfg, err := Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Agent.Socket) == "" {
		cfg.Agent.Socket = paths.AgentSocket()
	}
	cfg.Security.ExternalUsePolicy = policy
	return Save(paths.ConfigFile(), cfg)
}

func SetMasterPasswordInterval(paths Paths, interval string) error {
	cfg, err := Load(paths.ConfigFile())
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Agent.Socket) == "" {
		cfg.Agent.Socket = paths.AgentSocket()
	}
	cfg.Security.MasterPasswordInterval = NormalizeMasterPasswordInterval(interval)
	return Save(paths.ConfigFile(), cfg)
}
