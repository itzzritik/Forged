package sshrouting

import "strings"

type ProviderKind string

const (
	ProviderGitHub ProviderKind = "github"
	ProviderGitLab ProviderKind = "gitlab"
)

type Provider struct {
	Kind ProviderKind
	Host string
	User string
}

func DetectProvider(target Target) (Provider, bool) {
	if target.Kind != TargetGit {
		return Provider{}, false
	}
	host := strings.ToLower(strings.TrimSpace(target.Host))
	user := strings.ToLower(strings.TrimSpace(target.User))
	if user == "" {
		user = "git"
	}
	switch host {
	case "github.com", "ssh.github.com":
		return Provider{Kind: ProviderGitHub, Host: host, User: user}, true
	case "gitlab.com", "altssh.gitlab.com":
		return Provider{Kind: ProviderGitLab, Host: host, User: user}, true
	default:
		return Provider{}, false
	}
}

func providerRepoPath(target Target) string {
	path := strings.Trim(strings.TrimSpace(target.Owner+"/"+target.Repo), "/")
	if path == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(path), ".git") {
		path += ".git"
	}
	return path
}
