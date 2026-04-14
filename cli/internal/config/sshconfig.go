package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/sshrouting"
)

const (
	sshConfigMarker   = "# Added by Forged"
	sshIncludeComment = "# Forged SSH integration"
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
	return strings.Contains(content, includeLine(paths.SSHBaseInclude())) ||
		strings.Contains(content, sshConfigMarker) ||
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

	baseContent := sshrouting.RenderBaseConfig(paths.AgentSocket(), paths.SSHAdvancedConfig())
	if err := os.WriteFile(paths.SSHBaseInclude(), []byte(baseContent), 0o600); err != nil {
		return err
	}
	if _, err := os.Stat(paths.SSHAdvancedConfig()); os.IsNotExist(err) {
		if err := os.WriteFile(paths.SSHAdvancedConfig(), []byte(""), 0o600); err != nil {
			return err
		}
	}

	content, err := readConfigFile(configPath)
	if err != nil {
		return err
	}

	content = removeForgedInclude(content, paths.SSHBaseInclude())
	content = removeForgedBlock(content)

	block := strings.Join([]string{
		sshIncludeComment,
		includeLine(paths.SSHBaseInclude()),
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
		_ = os.Remove(paths.SSHBaseInclude())
		_ = os.Remove(paths.SSHAdvancedConfig())
		_ = os.RemoveAll(paths.SSHManagedDir())
		return nil
	}

	cleaned := removeForgedInclude(content, paths.SSHBaseInclude())
	cleaned = removeForgedBlock(cleaned)
	cleaned = strings.TrimRight(cleaned, "\n ")
	if cleaned != "" {
		cleaned += "\n"
	}

	if err := os.WriteFile(configPath, []byte(cleaned), 0o600); err != nil {
		return err
	}

	_ = os.Remove(paths.SSHBaseInclude())
	_ = os.Remove(paths.SSHAdvancedConfig())
	_ = os.RemoveAll(paths.SSHManagedDir())

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

func removeForgedInclude(content, includePath string) string {
	lines := strings.Split(content, "\n")
	include := includeLine(includePath)
	quotedInclude := fmt.Sprintf("Include %q", includePath)

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sshIncludeComment || trimmed == include || trimmed == quotedInclude {
			continue
		}
		result = append(result, line)
	}

	return trimTrailingBlankLines(strings.Join(result, "\n"))
}

func removeForgedBlock(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != sshConfigMarker {
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
