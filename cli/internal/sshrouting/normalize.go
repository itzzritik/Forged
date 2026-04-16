package sshrouting

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type TargetKind string

const (
	TargetGit TargetKind = "git"
	TargetSSH TargetKind = "ssh"
)

type Target struct {
	Kind      TargetKind
	Canonical string
	Host      string
	User      string
	Port      int
	Owner     string
	Repo      string
}

type PrepareInput struct {
	Host         string
	OriginalHost string
	User         string
	Port         string
}

func ParseCanonicalTarget(raw string) (Target, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Target{}, fmt.Errorf("parse canonical target: %w", err)
	}

	port, err := parsePort(u.Port(), 22)
	if err != nil {
		return Target{}, fmt.Errorf("parse port: %w", err)
	}

	target := Target{
		Canonical: raw,
		Host:      strings.ToLower(u.Hostname()),
		User:      strings.ToLower(u.User.Username()),
		Port:      port,
	}

	switch u.Scheme {
	case "ssh":
		target.Kind = TargetSSH
		return target, nil
	case "git+ssh":
		target.Kind = TargetGit
		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return Target{}, fmt.Errorf("canonical git target missing owner/repo: %q", raw)
		}
		target.Owner = parts[0]
		target.Repo = parts[1]
		return target, nil
	default:
		return Target{}, fmt.Errorf("unsupported canonical target scheme %q", u.Scheme)
	}
}

func ResolveSSHTarget(input PrepareInput) (Target, error) {
	host := strings.TrimSpace(input.Host)
	user := strings.TrimSpace(input.User)
	if host == "" || user == "" {
		return Target{}, fmt.Errorf("ssh target requires host and user")
	}

	port, err := parsePort(strings.TrimSpace(input.Port), 22)
	if err != nil {
		return Target{}, fmt.Errorf("parse ssh port: %w", err)
	}

	return Target{
		Kind:      TargetSSH,
		Canonical: fmt.Sprintf("ssh://%s@%s:%d", strings.ToLower(user), strings.ToLower(host), port),
		Host:      strings.ToLower(host),
		User:      strings.ToLower(user),
		Port:      port,
	}, nil
}

func parsePort(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return port, nil
}

type Remote struct {
	Raw      string
	User     string
	Host     string
	Owner    string
	Repo     string
	RouteKey string
}

func NormalizeRemote(raw string) (Remote, error) {
	target, err := normalizeGitRemote(raw)
	if err != nil {
		return Remote{}, err
	}
	return remoteFromTarget(raw, target), nil
}

func remoteFromTarget(raw string, target Target) Remote {
	user := strings.ToLower(target.User)
	host := strings.ToLower(target.Host)
	owner := strings.ToLower(target.Owner)
	repo := strings.ToLower(target.Repo)
	return Remote{
		Raw:      raw,
		User:     user,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
		RouteKey: fmt.Sprintf("%s@%s:%s/%s", user, host, owner, repo),
	}
}
