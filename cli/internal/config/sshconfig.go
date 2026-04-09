package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	sshConfigPath := SSHConfigPath()
	marker := "# Added by Forged"
	agentLine := fmt.Sprintf("    IdentityAgent %q", paths.AgentSocket())
	block := fmt.Sprintf("%s\nHost *\n%s\n", marker, agentLine)

	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0700); err != nil {
		return err
	}

	data, err := os.ReadFile(sshConfigPath)
	if err != nil {
		return os.WriteFile(sshConfigPath, []byte(block), 0600)
	}

	content := string(data)

	if strings.Contains(content, paths.AgentSocket()) {
		return nil
	}

	cleaned := removeIdentityAgentLines(content, marker)
	result := strings.TrimRight(cleaned, "\n") + "\n\n" + block
	return os.WriteFile(sshConfigPath, []byte(result), 0600)
}

func DisableSSHAgent(paths Paths) error {
	sshConfigPath := SSHConfigPath()
	data, err := os.ReadFile(sshConfigPath)
	if err != nil {
		return nil
	}

	marker := "# Added by Forged"
	content := string(data)

	if !strings.Contains(content, marker) && !strings.Contains(content, paths.AgentSocket()) {
		return nil
	}

	cleaned := removeIdentityAgentLines(content, marker)

	for strings.Contains(cleaned, marker) {
		cleaned = strings.ReplaceAll(cleaned, marker+"\n", "")
		cleaned = strings.ReplaceAll(cleaned, marker, "")
	}

	result := strings.TrimRight(cleaned, "\n ") + "\n"
	return os.WriteFile(sshConfigPath, []byte(result), 0600)
}

func removeIdentityAgentLines(content, marker string) string {
	var cleaned []string
	lines := strings.Split(content, "\n")
	skipNextIdentityAgent := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == marker {
			continue
		}

		if skipNextIdentityAgent {
			if strings.HasPrefix(trimmed, "IdentityAgent") {
				skipNextIdentityAgent = false
				continue
			}
			skipNextIdentityAgent = false
		}

		if strings.HasPrefix(trimmed, "Host *") {
			skipNextIdentityAgent = true
		}

		if strings.HasPrefix(trimmed, "IdentityAgent") {
			continue
		}

		cleaned = append(cleaned, line)
	}

	var final []string
	for i, line := range cleaned {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Host *") {
			nextNonEmpty := ""
			for j := i + 1; j < len(cleaned); j++ {
				t := strings.TrimSpace(cleaned[j])
				if t != "" {
					nextNonEmpty = t
					break
				}
			}
			if nextNonEmpty == "" || strings.HasPrefix(nextNonEmpty, "Host ") || strings.HasPrefix(nextNonEmpty, "#") || strings.HasPrefix(nextNonEmpty, "Include") {
				continue
			}
		}
		final = append(final, line)
	}

	return strings.Join(final, "\n")
}
