package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/platform"
)

const (
	legacySSHConfigMarker = "# Added by Forged"
	sshIncludeComment     = "# Forged SSH integration"
	sshAgentComment       = "# Forged SSH Agent"
	sshRoutesComment      = "# Forged SSH Routing"
)

func SSHConfigPath() string {
	return DefaultPaths().SSHUserConfig()
}

func IsSSHAgentEnabled(paths Paths) bool {
	data, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		return false
	}
	includes := forgedIncludeLines(paths)

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if _, ok := includes[trimmed]; ok {
			return true
		}

		if strings.Contains(trimmed, "IdentityAgent") && strings.Contains(trimmed, paths.AgentSocket()) {
			return true
		}
	}

	return false
}

func EnableSSHAgent(paths Paths) error {
	return withSSHConfigLock(paths, func() error {
		configPath := paths.SSHUserConfig()
		if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
			return fmt.Errorf("Creating SSH config directory: %w", err)
		}
		if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
			return fmt.Errorf("Creating managed SSH directory: %w", err)
		}

		if err := cleanupLegacySSHArtifacts(paths); err != nil {
			return err
		}
		if err := ensureManagedSSHConfigLocked(paths); err != nil {
			return err
		}

		content, err := readConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("Reading SSH config: %w", err)
		}
		content = removeForgedIncludes(content, paths)
		content = removeLegacyForgedBlock(content)

		block := strings.Join([]string{
			sshIncludeComment,
			includeLine(paths.SSHManagedConfig()),
		}, "\n")
		if err := writeSSHFileAtomic(configPath, []byte(insertForgedInclude(content, block))); err != nil {
			return fmt.Errorf("Writing SSH config: %w", err)
		}
		return SetAgentDisabled(paths, false)
	})
}

func DisableSSHAgent(paths Paths) error {
	return withSSHConfigLock(paths, func() error {
		if err := cleanupLegacySSHArtifacts(paths); err != nil {
			return err
		}

		configPath := paths.SSHUserConfig()
		content, err := readConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("Reading SSH config: %w", err)
		}
		if content == "" {
			if _, err := os.Stat(paths.SSHManagedConfig()); os.IsNotExist(err) {
				return SetAgentDisabled(paths, true)
			}
		}

		cleaned := removeForgedIncludes(content, paths)
		cleaned = removeLegacyForgedBlock(cleaned)
		block := strings.Join([]string{
			sshIncludeComment,
			"# " + includeLine(paths.SSHManagedConfig()),
		}, "\n")
		if err := writeSSHFileAtomic(configPath, []byte(insertForgedInclude(cleaned, block))); err != nil {
			return fmt.Errorf("Writing SSH config: %w", err)
		}
		return SetAgentDisabled(paths, true)
	})
}

func includeLine(path string) string {
	path = filepath.ToSlash(path)
	path = strings.ReplaceAll(path, "%", "%%")
	return fmt.Sprintf("Include %q", path)
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
	includes := forgedIncludeLines(paths)

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

func forgedIncludeLines(paths Paths) map[string]struct{} {
	includes := make(map[string]struct{})
	for _, path := range []string{paths.SSHManagedConfig(), paths.LegacySSHBaseInclude()} {
		forward := filepath.ToSlash(path)
		for _, line := range []string{
			"Include " + path,
			fmt.Sprintf("Include %q", path),
			"Include " + forward,
			fmt.Sprintf("Include %q", forward),
			includeLine(path),
		} {
			includes[line] = struct{}{}
			includes["# "+line] = struct{}{}
		}
	}
	return includes
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
		lines = append(lines, "    PermitLocalCommand yes")
		lines = append(lines, "", sshRoutesComment, routes)
	}

	return strings.Join(lines, "\n") + "\n"
}

func cleanupLegacySSHArtifacts(paths Paths) error {
	_ = os.RemoveAll(paths.LegacySSHManagedDir())
	_ = os.Remove(filepath.Join(paths.StateDir, "ssh-routing.json"))
	_ = os.Remove(paths.SSHLegacyAdvancedConfig())
	_ = os.Remove(filepath.Join(paths.SSHManagedDir(), "routing.json"))
	return nil
}

func ensureManagedSSHConfigLocked(paths Paths) error {
	if _, err := os.Stat(paths.SSHManagedConfig()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Inspecting managed SSH config: %w", err)
	}

	baseContent := RenderManagedSSHConfig(paths, "")
	return writeSSHFileAtomic(paths.SSHManagedConfig(), []byte(baseContent))
}

func WriteManagedSSHConfig(paths Paths, content string) error {
	return withSSHConfigLock(paths, func() error {
		if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
			return fmt.Errorf("Creating managed SSH directory: %w", err)
		}
		return writeSSHFileAtomic(paths.SSHManagedConfig(), []byte(content))
	})
}

func withSSHConfigLock(paths Paths, fn func() error) error {
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("Creating config directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(paths.ConfigDir, ".ssh-config.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("Opening SSH config lock: %w", err)
	}
	defer lock.Close()
	if err := platform.LockFileWait(lock); err != nil {
		return fmt.Errorf("Locking SSH config: %w", err)
	}
	defer platform.UnlockFile(lock)
	return fn()
}

func writeSSHFileAtomic(path string, data []byte) error {
	target, err := resolveSSHWritePath(path)
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
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("Replacing %s: %w", target, err)
	}
	return nil
}

func resolveSSHWritePath(path string) (string, error) {
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
	return "", fmt.Errorf("Too many SSH config symlinks")
}
