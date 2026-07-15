package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	if err := paths.ValidateRuntimePaths(); err != nil {
		return false
	}
	data, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		return false
	}
	includes := forgedIncludeLines(paths)
	agentSocket := paths.AgentSocket()

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if _, ok := includes[trimmed]; ok {
			return true
		}

		if strings.Contains(trimmed, "IdentityAgent") && agentSocket != "" && strings.Contains(trimmed, agentSocket) {
			return true
		}
	}

	return false
}

func hasDisabledForgedInclude(paths Paths) bool {
	data, err := os.ReadFile(paths.SSHUserConfig())
	if err != nil {
		return false
	}
	includes := forgedIncludeLines(paths)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if _, ok := includes[trimmed]; ok {
				return true
			}
		}
	}
	return false
}

func EnableSSHAgent(paths Paths) error {
	if err := paths.ValidateRuntimePaths(); err != nil {
		return err
	}
	return withSSHConfigLock(paths, func() error {
		configPath := paths.SSHUserConfig()
		content, err := readConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("Reading SSH config: %w", err)
		}
		content, err = removeLegacyForgedBlock(content, paths)
		if err != nil {
			return fmt.Errorf("Migrating legacy SSH config: %w", err)
		}
		content = removeForgedIncludes(content, paths)

		if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
			return fmt.Errorf("Creating SSH config directory: %w", err)
		}
		if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
			return fmt.Errorf("Creating managed SSH directory: %w", err)
		}
		if err := ensureManagedSSHConfigLocked(paths); err != nil {
			return err
		}

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
		configPath := paths.SSHUserConfig()
		content, err := readConfigFile(configPath)
		if err != nil {
			return fmt.Errorf("Reading SSH config: %w", err)
		}
		cleaned, err := removeLegacyForgedBlock(content, paths)
		if err != nil {
			return fmt.Errorf("Migrating legacy SSH config: %w", err)
		}
		cleaned = removeForgedIncludes(cleaned, paths)
		if content == "" {
			if _, err := os.Stat(paths.SSHManagedConfig()); os.IsNotExist(err) {
				return SetAgentDisabled(paths, true)
			}
		}

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
	pathsToRemove := append(legacyManagedSSHConfigPaths(paths), paths.LegacySSHBaseInclude())
	for _, path := range pathsToRemove {
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

func removeLegacyForgedBlock(content string, paths Paths) (string, error) {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	found := false

	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) != legacySSHConfigMarker {
			result = append(result, lines[i])
			i++
			continue
		}

		if found {
			return "", legacyForgedBlockError(i, paths)
		}
		end, ok := legacyForgedBlockEnd(lines, i, paths)
		if !ok {
			return "", legacyForgedBlockError(i, paths)
		}
		found = true
		i = end
	}

	return trimTrailingBlankLines(strings.Join(result, "\n")), nil
}

func legacyForgedBlockError(line int, paths Paths) error {
	return fmt.Errorf("legacy Forged SSH block at line %d was modified; refusing to change %s", line+1, paths.SSHUserConfig())
}

func legacyForgedBlockEnd(lines []string, marker int, paths Paths) (int, bool) {
	next := marker + 1
	includedBeforeHost := false
	if next < len(lines) && isLegacyForgedQuotedInclude(lines[next], paths) {
		includedBeforeHost = true
		next++
	}
	if next+1 >= len(lines) || strings.TrimSpace(lines[next]) != "Host *" || !isLegacyForgedIdentityAgent(lines[next+1], paths) {
		return 0, false
	}

	includedAfterHost := false
	for next += 2; next < len(lines); next++ {
		trimmed := strings.TrimSpace(lines[next])
		if trimmed == "" {
			continue
		}
		if isLegacyForgedSectionBoundary(trimmed) {
			return next, true
		}
		if !includedBeforeHost && !includedAfterHost && isLegacyForgedUnquotedInclude(lines[next], paths) {
			includedAfterHost = true
			continue
		}
		return 0, false
	}
	return len(lines), true
}

func isLegacyForgedIdentityAgent(line string, paths Paths) bool {
	for _, socket := range legacyForgedAgentSockets(paths) {
		if socket != "" && line == fmt.Sprintf("    IdentityAgent %q", socket) {
			return true
		}
	}
	return false
}

func isLegacyForgedQuotedInclude(line string, paths Paths) bool {
	for _, path := range legacySSHAdvancedConfigPaths(paths) {
		if line == fmt.Sprintf("Include %q", path) {
			return true
		}
	}
	return false
}

func isLegacyForgedUnquotedInclude(line string, paths Paths) bool {
	for _, path := range legacySSHAdvancedConfigPaths(paths) {
		if line == "Include "+path {
			return true
		}
	}
	return false
}

func isLegacyForgedSectionBoundary(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && (strings.EqualFold(fields[0], "host") || strings.EqualFold(fields[0], "match"))
}

func legacyForgedAgentSockets(paths Paths) []string {
	sockets := []string{paths.AgentSocket()}
	home, err := os.UserHomeDir()
	if err != nil {
		return sockets
	}

	switch runtime.GOOS {
	case "darwin":
		sockets = append(sockets,
			filepath.Join(home, ".forged", "agent.sock"),
			filepath.Join(home, ".forged", "runtime", "agent.sock"),
		)
	case "linux":
		sockets = append(sockets, filepath.Join("/run", "user", uidStr(), "forged", "agent.sock"))
	case "windows":
		sockets = append(sockets, `\\.\pipe\forged-agent`)
	}
	return sockets
}

func legacyManagedSSHConfigPaths(paths Paths) []string {
	configs := make([]string, 0, len(legacySSHManagedDirs(paths)))
	for _, dir := range legacySSHManagedDirs(paths) {
		configs = append(configs, filepath.Join(dir, "forged.conf"))
	}
	return configs
}

func legacySSHAdvancedConfigPaths(paths Paths) []string {
	configs := make([]string, 0, len(legacySSHManagedDirs(paths))+1)
	for _, dir := range legacySSHManagedDirs(paths) {
		configs = append(configs, filepath.Join(dir, "config"))
	}
	configs = append(configs, filepath.Join(paths.LegacySSHManagedDir(), "config"))
	return configs
}

func legacySSHManagedDirs(paths Paths) []string {
	dirs := []string{paths.SSHManagedDir()}
	home, err := os.UserHomeDir()
	if err != nil {
		return dirs
	}

	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs,
			filepath.Join(home, ".forged", "ssh"),
			filepath.Join(home, ".forged", "config", "ssh"),
		)
	case "linux":
		configHome := envOrDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		dirs = append(dirs,
			filepath.Join(home, ".forged", "ssh"),
			filepath.Join(configHome, "forged", "ssh"),
		)
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			if profile := os.Getenv("USERPROFILE"); profile != "" {
				appData = filepath.Join(profile, "AppData", "Roaming")
			}
		}
		if appData != "" {
			dirs = append(dirs, filepath.Join(appData, "forged", "ssh"))
		}
	}
	return dirs
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

	if !platform.SSHRoutingSupported() {
		routes = ""
	}
	routes = strings.TrimSpace(routes)
	if routes != "" {
		lines = append(lines, "    PermitLocalCommand yes")
		lines = append(lines, "", sshRoutesComment, routes)
	}

	return strings.Join(lines, "\n") + "\n"
}

// EnsureAgentOnlyManagedSSHConfig removes the product-owned routing section while preserving the managed agent section.
func EnsureAgentOnlyManagedSSHConfig(paths Paths) error {
	if err := paths.ValidateRuntimePaths(); err != nil {
		return err
	}
	return withSSHConfigLock(paths, func() error {
		if err := os.MkdirAll(paths.SSHManagedDir(), 0o700); err != nil {
			return fmt.Errorf("Creating managed SSH directory: %w", err)
		}
		content, err := readConfigFile(paths.SSHManagedConfig())
		if err != nil {
			return fmt.Errorf("reading managed SSH config: %w", err)
		}
		return ensureAgentOnlyManagedSSHConfigLocked(paths, content)
	})
}

func ensureManagedSSHConfigLocked(paths Paths) error {
	path := paths.SSHManagedConfig()
	content, err := readConfigFile(path)
	if err != nil {
		return fmt.Errorf("reading managed SSH config: %w", err)
	}
	if !platform.SSHRoutingSupported() {
		return ensureAgentOnlyManagedSSHConfigLocked(paths, content)
	}
	if content == "" {
		return writeSSHFileAtomic(path, []byte(RenderManagedSSHConfig(paths, "")))
	}

	updated, err := updateManagedSSHIdentityAgent(content, paths.AgentSocket())
	if err != nil {
		return fmt.Errorf("updating managed SSH config: %w", err)
	}
	if updated == content {
		return nil
	}
	return writeSSHFileAtomic(path, []byte(updated))
}

func ensureAgentOnlyManagedSSHConfigLocked(paths Paths, content string) error {
	path := paths.SSHManagedConfig()
	if content == "" {
		return writeSSHFileAtomic(path, []byte(RenderManagedSSHConfig(paths, "")))
	}

	updated, err := updateManagedSSHIdentityAgent(content, paths.AgentSocket())
	if err != nil {
		return fmt.Errorf("updating managed SSH config: %w", err)
	}
	updated, err = removeManagedSSHRoutingTail(updated)
	if err != nil {
		return err
	}
	if updated == content {
		return nil
	}
	return writeSSHFileAtomic(path, []byte(updated))
}

func removeManagedSSHRoutingTail(content string) (string, error) {
	lines := strings.Split(content, "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	marker := -1
	for i := 0; i < end; i++ {
		if strings.TrimSuffix(lines[i], "\r") != sshRoutesComment {
			continue
		}
		if marker >= 0 {
			return "", fmt.Errorf("managed SSH routing tail is ambiguous")
		}
		marker = i
	}
	if marker < 0 {
		return content, nil
	}
	permit := marker - 1
	for permit >= 0 && strings.TrimSpace(lines[permit]) == "" {
		permit--
	}
	if permit < 0 || strings.TrimSuffix(lines[permit], "\r") != "    PermitLocalCommand yes" {
		return "", fmt.Errorf("managed SSH routing tail is missing its generated PermitLocalCommand")
	}

	prefix := trimTrailingBlankLines(strings.Join(lines[:permit], "\n"))
	if prefix == "" {
		return "", fmt.Errorf("managed SSH routing tail has no managed config prefix")
	}
	return prefix + "\n", nil
}

func updateManagedSSHIdentityAgent(content, agentSocket string) (string, error) {
	lines := strings.Split(content, "\n")
	marker := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == sshAgentComment {
			marker = i
			break
		}
	}
	if marker < 0 {
		return "", fmt.Errorf("missing %q marker", sshAgentComment)
	}

	host := -1
	for i := marker + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 1 && strings.EqualFold(fields[0], "Host") {
			for _, pattern := range fields[1:] {
				if strings.HasPrefix(pattern, "#") {
					break
				}
				if pattern == "*" {
					host = i
					break
				}
			}
		}
		break
	}
	if host < 0 {
		return "", fmt.Errorf("missing Host block for *")
	}

	end := len(lines)
	for i := host + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && (strings.EqualFold(fields[0], "Host") || strings.EqualFold(fields[0], "Match")) {
			end = i
			break
		}
	}
	identity := fmt.Sprintf("    IdentityAgent %q", agentSocket)
	for i := host + 1; i < end; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) > 0 && strings.EqualFold(fields[0], "IdentityAgent") {
			lines[i] = identity
			return strings.Join(lines, "\n"), nil
		}
	}

	lines = append(lines[:host+1], append([]string{identity}, lines[host+1:]...)...)
	return strings.Join(lines, "\n"), nil
}

func WriteManagedSSHConfig(paths Paths, content string) error {
	if err := paths.ValidateRuntimePaths(); err != nil {
		return err
	}
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
	return writePrivateFileAtomic(path, data)
}
