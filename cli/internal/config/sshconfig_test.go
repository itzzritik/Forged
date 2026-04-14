package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnableSSHAgentWritesSingleIncludeAndBaseFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := DefaultPaths()
	if err := EnableSSHAgent(paths); err != nil {
		t.Fatalf("enable ssh agent: %v", err)
	}

	mainConfig, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		t.Fatalf("read main config: %v", err)
	}
	if !strings.Contains(string(mainConfig), "Include "+paths.SSHBaseInclude()) {
		t.Fatalf("missing include: %s", string(mainConfig))
	}

	baseConfig, err := os.ReadFile(paths.SSHBaseInclude())
	if err != nil {
		t.Fatalf("read base include: %v", err)
	}
	if !strings.Contains(string(baseConfig), "Host *") || !strings.Contains(string(baseConfig), "IdentityAgent") {
		t.Fatalf("unexpected base config: %s", string(baseConfig))
	}
}

func TestEnableSSHAgentMigratesOldInlineForgedBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := DefaultPaths()
	if err := os.MkdirAll(filepath.Dir(paths.SSHUserConfig()), 0o700); err != nil {
		t.Fatal(err)
	}

	legacy := "# Added by Forged\nHost *\n    IdentityAgent \"/tmp/forged-agent.sock\"\n"
	if err := os.WriteFile(paths.SSHUserConfig(), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnableSSHAgent(paths); err != nil {
		t.Fatalf("enable ssh agent: %v", err)
	}

	mainConfig, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mainConfig), "# Added by Forged") {
		t.Fatalf("legacy inline block still present: %s", string(mainConfig))
	}
	if !strings.Contains(string(mainConfig), "Include "+paths.SSHBaseInclude()) {
		t.Fatalf("expected include after migration: %s", string(mainConfig))
	}
}

func TestEnableSSHAgentPlacesIncludeBeforeExistingHostBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := DefaultPaths()
	if err := os.MkdirAll(filepath.Dir(paths.SSHUserConfig()), 0o700); err != nil {
		t.Fatal(err)
	}

	existing := strings.Join([]string{
		"Include /Users/example/.colima/ssh_config",
		"",
		"Host 144.24.124.129",
		"  HostName 144.24.124.129",
		"  User ubuntu",
		"",
	}, "\n")
	if err := os.WriteFile(paths.SSHUserConfig(), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnableSSHAgent(paths); err != nil {
		t.Fatalf("enable ssh agent: %v", err)
	}

	mainConfig, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		t.Fatal(err)
	}

	got := string(mainConfig)
	includeIdx := strings.Index(got, "Include "+paths.SSHBaseInclude())
	hostIdx := strings.Index(got, "Host 144.24.124.129")
	if includeIdx < 0 {
		t.Fatalf("missing forged include: %s", got)
	}
	if hostIdx < 0 {
		t.Fatalf("missing existing host block: %s", got)
	}
	if includeIdx > hostIdx {
		t.Fatalf("forged include should be inserted before host blocks: %s", got)
	}
}

func TestDisableSSHAgentRemovesIncludeButLeavesUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := DefaultPaths()
	if err := os.MkdirAll(filepath.Dir(paths.SSHUserConfig()), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "Host github.com\n  User git\n"
	if err := os.WriteFile(paths.SSHUserConfig(), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := EnableSSHAgent(paths); err != nil {
		t.Fatal(err)
	}
	if err := DisableSSHAgent(paths); err != nil {
		t.Fatal(err)
	}

	mainConfig, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mainConfig), "Include "+paths.SSHBaseInclude()) {
		t.Fatalf("include should be removed: %s", string(mainConfig))
	}
	if !strings.Contains(string(mainConfig), "Host github.com") {
		t.Fatalf("user config should be preserved: %s", string(mainConfig))
	}
}
