package sshrouting

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type RepoContext struct {
	Host      string
	Owner     string
	Repo      string
	RemoteURL string
}

func DetectRepoContext(cwd string) (RepoContext, bool) {
	configPath, ok := locateGitConfig(cwd)
	if !ok {
		return RepoContext{}, false
	}

	remoteURL, ok := readOriginURL(configPath)
	if !ok {
		return RepoContext{}, false
	}

	host, owner, repo, ok := parseRemoteURL(remoteURL)
	if !ok {
		return RepoContext{}, false
	}

	return RepoContext{
		Host:      host,
		Owner:     owner,
		Repo:      repo,
		RemoteURL: remoteURL,
	}, true
}

func locateGitConfig(cwd string) (string, bool) {
	dir := cwd
	for {
		if configPath, ok := gitConfigInDir(dir); ok {
			return configPath, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func gitConfigInDir(dir string) (string, bool) {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		configPath := filepath.Join(gitPath, "config")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, true
		}
		return "", false
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(strings.ToLower(line), "gitdir:") {
		return "", false
	}

	gitDir := strings.TrimSpace(line[len("gitdir:"):])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	gitDir = filepath.Clean(gitDir)

	worktreeConfig := filepath.Join(gitDir, "config")
	if _, err := os.Stat(worktreeConfig); err == nil {
		return worktreeConfig, true
	}

	commonConfig := filepath.Clean(filepath.Join(gitDir, "..", "..", "config"))
	if _, err := os.Stat(commonConfig); err == nil {
		return commonConfig, true
	}

	return "", false
}

func readOriginURL(configPath string) (string, bool) {
	file, err := os.Open(configPath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inOrigin := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inOrigin = strings.EqualFold(line, `[remote "origin"]`)
			continue
		}

		if !inOrigin {
			continue
		}

		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "url") {
			value := strings.TrimSpace(parts[1])
			if value != "" {
				return value, true
			}
		}
	}

	return "", false
}

func parseRemoteURL(raw string) (host, owner, repo string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", false
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", "", "", false
		}
		host = parsed.Hostname()
		path := strings.TrimPrefix(parsed.Path, "/")
		return splitOwnerRepo(host, path)
	}

	at := strings.Index(raw, "@")
	colon := strings.Index(raw, ":")
	if at >= 0 && colon > at {
		host = raw[at+1 : colon]
		path := raw[colon+1:]
		return splitOwnerRepo(host, path)
	}

	return "", "", "", false
}

func splitOwnerRepo(host, path string) (string, string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", false
	}

	owner := parts[0]
	repo := parts[len(parts)-1]
	repo = strings.TrimSuffix(repo, ".git")
	if host == "" || owner == "" || repo == "" {
		return "", "", "", false
	}

	return host, owner, repo, true
}
