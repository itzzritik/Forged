package hostmatch

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type SSHHostEntry struct {
	Host         string
	IdentityFile string
}

func ParseSSHConfig(path string) ([]SSHHostEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []SSHHostEntry
	var current *SSHHostEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value := splitDirective(line)
		key = strings.ToLower(key)

		switch key {
		case "host":
			if current != nil && current.IdentityFile != "" {
				entries = append(entries, *current)
			}
			current = &SSHHostEntry{Host: value}
		case "identityfile":
			if current != nil {
				current.IdentityFile = expandPath(value)
			}
		}
	}

	if current != nil && current.IdentityFile != "" {
		entries = append(entries, *current)
	}

	return entries, scanner.Err()
}

func FindSSHKeys(configPath string) []string {
	entries, err := ParseSSHConfig(configPath)
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var paths []string
	for _, e := range entries {
		p := e.IdentityFile
		if seen[p] {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths
}

func DiscoverSSHKeys() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	candidates := []string{
		"id_ed25519",
		"id_ecdsa",
		"id_rsa",
		"id_dsa",
	}

	var found []string
	for _, name := range candidates {
		p := filepath.Join(sshDir, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}

	configKeys := FindSSHKeys(filepath.Join(sshDir, "config"))
	seen := map[string]bool{}
	for _, p := range found {
		seen[p] = true
	}
	for _, p := range configKeys {
		if !seen[p] {
			found = append(found, p)
		}
	}

	return found
}

func splitDirective(line string) (string, string) {
	line = strings.TrimSpace(line)

	if idx := strings.Index(line, "="); idx != -1 {
		return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
	}

	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		return parts[0], strings.TrimSpace(parts[1])
	}
	return parts[0], ""
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
