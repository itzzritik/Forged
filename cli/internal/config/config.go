package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Agent AgentConfig `toml:"agent"`
	Sync  SyncConfig  `toml:"sync"`
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

func Load(path string) (Config, error) {
	var cfg Config
	cfg.Agent.LogLevel = "info"

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}

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

	return os.WriteFile(path, []byte(body.String()), 0o600)
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
