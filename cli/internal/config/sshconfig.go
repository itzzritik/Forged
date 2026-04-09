package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sshConfigMarker = "# Added by Forged"

func SSHConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func IsSSHAgentEnabled(paths Paths) bool {
	data, err := os.ReadFile(SSHConfigPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(data), paths.AgentSocket())
}

func EnableSSHAgent(paths Paths) error {
	configPath := SSHConfigPath()
	agentLine := fmt.Sprintf("    IdentityAgent %q", paths.AgentSocket())
	block := fmt.Sprintf("%s\nHost *\n%s\n", sshConfigMarker, agentLine)

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return os.WriteFile(configPath, []byte(block), 0600)
	}

	content := string(data)
	if strings.Contains(content, paths.AgentSocket()) {
		return nil
	}

	cleaned := removeForgedBlock(content)
	cleaned = removeWildcardIdentityAgent(cleaned)
	result := strings.TrimRight(cleaned, "\n") + "\n\n" + block
	return os.WriteFile(configPath, []byte(result), 0600)
}

func DisableSSHAgent(paths Paths) error {
	configPath := SSHConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	content := string(data)
	if !strings.Contains(content, sshConfigMarker) && !strings.Contains(content, paths.AgentSocket()) {
		return nil
	}

	cleaned := removeForgedBlock(content)
	result := strings.TrimRight(cleaned, "\n ") + "\n"
	return os.WriteFile(configPath, []byte(result), 0600)
}

func removeForgedBlock(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inForgedBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == sshConfigMarker {
			inForgedBlock = true
			continue
		}

		if inForgedBlock {
			if strings.HasPrefix(trimmed, "Host ") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "Include") || trimmed == "" {
				if trimmed != "" {
					inForgedBlock = false
					result = append(result, line)
				}
				continue
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func removeWildcardIdentityAgent(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inWildcardHost := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Host ") {
			inWildcardHost = trimmed == "Host *"
		}

		if inWildcardHost && strings.HasPrefix(trimmed, "IdentityAgent") {
			continue
		}

		result = append(result, line)
	}

	var final []string
	for i, line := range result {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Host *" {
			hasContent := false
			for j := i + 1; j < len(result); j++ {
				t := strings.TrimSpace(result[j])
				if t == "" {
					continue
				}
				if strings.HasPrefix(t, "Host ") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "Include") {
					break
				}
				hasContent = true
				break
			}
			if !hasContent {
				continue
			}
		}
		final = append(final, line)
	}

	return strings.Join(final, "\n")
}
