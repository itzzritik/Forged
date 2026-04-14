package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Agent AgentConfig `toml:"agent"`
	Sync  SyncConfig  `toml:"sync"`
}

type AgentConfig struct {
	Socket   string `toml:"socket"`
	LogLevel string `toml:"log_level"`
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
