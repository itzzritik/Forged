package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	legacySSHConfigMarker = "# Added by Forged"
	sshIncludeComment = "# Forged SSH integration"
	sshAgentComment   = "# Forged SSH Agent"
	sshRoutesComment  = "# Forged Git Remote Routes"
)

func SSHConfigPath() string {
	return DefaultPaths().SSHUserConfig()
}

func IsSSHAgentEnabled(paths Paths) bool {
	data, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		return false
	}

	content := string(data)
	return strings.Contains(content, includeLine(paths.SSHManagedConfig())) ||
		strings.Contains(content, includeLine(paths.LegacySSHBaseInclude())) ||
		strings.Contains(content, paths.AgentSocket())
}

func EnableSSHAgent(paths Paths) error {
	configPath := paths.SSHUserConfig()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
		return err
	}

	if err := cleanupLegacySSHArtifacts(paths); err != nil {
		return err
	}
	baseContent := RenderManagedSSHConfig(paths, "")
	if err := os.WriteFile(paths.SSHManagedConfig(), []byte(baseContent), 0o600); err != nil {
		return err
	}

	content, err := readConfigFile(configPath)
	if err != nil {
		return err
	}

	content = removeForgedIncludes(content, paths)
	content = removeLegacyForgedBlock(content)

	block := strings.Join([]string{
		sshIncludeComment,
		includeLine(paths.SSHManagedConfig()),
	}, "\n")

	body := insertForgedInclude(content, block)

	return os.WriteFile(configPath, []byte(body), 0o600)
}

func DisableSSHAgent(paths Paths) error {
	configPath := paths.SSHUserConfig()
	content, err := readConfigFile(configPath)
	if err != nil {
		return err
	}
	if content == "" {
		_ = cleanupAllSSHArtifacts(paths)
		return nil
	}

	cleaned := removeForgedIncludes(content, paths)
	cleaned = removeLegacyForgedBlock(cleaned)
	cleaned = strings.TrimRight(cleaned, "\n ")
	if cleaned != "" {
		cleaned += "\n"
	}

	if err := os.WriteFile(configPath, []byte(cleaned), 0o600); err != nil {
		return err
	}

	_ = cleanupAllSSHArtifacts(paths)

	return nil
}

func includeLine(path string) string {
	return "Include " + path
}

func readConfigFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if os.IsNotExist(err) {
		return "", nil
	}
	return "", err
}

func removeForgedIncludes(content string, paths Paths) string {
	lines := strings.Split(content, "\n")
	includes := map[string]struct{}{
		includeLine(paths.SSHManagedConfig()):                   {},
		includeLine(paths.LegacySSHBaseInclude()):               {},
		fmt.Sprintf("Include %q", paths.SSHManagedConfig()):     {},
		fmt.Sprintf("Include %q", paths.LegacySSHBaseInclude()): {},
	}

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sshIncludeComment {
			continue
		}
		if _, ok := includes[trimmed]; ok {
			continue
		}
		result = append(result, line)
	}

	return trimTrailingBlankLines(strings.Join(result, "\n"))
}

func removeLegacyForgedBlock(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != legacySSHConfigMarker {
			result = append(result, lines[i])
			continue
		}

		for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			i++
		}
		if i+1 >= len(lines) {
			break
		}
		if strings.TrimSpace(lines[i+1]) != "Host *" {
			continue
		}

		i++
		for i+1 < len(lines) {
			next := lines[i+1]
			trimmed := strings.TrimSpace(next)
			if trimmed == "" {
				i++
				continue
			}
			if strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t") {
				i++
				continue
			}
			break
		}
	}

	return trimTrailingBlankLines(strings.Join(result, "\n"))
}

func trimTrailingBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[:end], "\n")
}

func insertForgedInclude(content, block string) string {
	body := strings.TrimRight(content, "\n")
	if body == "" {
		return block + "\n"
	}

	lines := strings.Split(body, "\n")
	insertAt := 0
	for insertAt < len(lines) {
		trimmed := strings.TrimSpace(lines[insertAt])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			insertAt++
			continue
		}
		break
	}

	prefix := strings.TrimRight(strings.Join(lines[:insertAt], "\n"), "\n")
	suffix := strings.TrimLeft(strings.Join(lines[insertAt:], "\n"), "\n")

	parts := make([]string, 0, 3)
	if prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, block)
	if suffix != "" {
		parts = append(parts, suffix)
	}

	return strings.Join(parts, "\n\n") + "\n"
}

func RenderManagedSSHConfig(paths Paths, routes string) string {
	lines := []string{
		sshAgentComment,
		"Host *",
		fmt.Sprintf("    IdentityAgent %q", paths.AgentSocket()),
	}

	routes = strings.TrimSpace(routes)
	if routes != "" {
		lines = append(lines, "", sshRoutesComment, routes)
	}

	return strings.Join(lines, "\n") + "\n"
}

func cleanupLegacySSHArtifacts(paths Paths) error {
	_ = os.RemoveAll(paths.LegacySSHManagedDir())
	_ = os.Remove(filepath.Join(paths.StateDir, "ssh-routing.json"))
	_ = os.Remove(paths.SSHLegacyAdvancedConfig())
	return nil
}

func cleanupAllSSHArtifacts(paths Paths) error {
	if err := cleanupLegacySSHArtifacts(paths); err != nil {
		return err
	}
	_ = os.Remove(paths.SSHManagedConfig())
	_ = os.RemoveAll(paths.SSHManagedDir())
	return nil
}
