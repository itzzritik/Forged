package sshrouting

import (
	"fmt"
	"net/url"
	"strings"
)

type Remote struct {
	Raw      string
	User     string
	Host     string
	Owner    string
	Repo     string
	RouteKey string
}

func NormalizeRemote(raw string) (Remote, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Remote{}, fmt.Errorf("empty remote")
	}

	if strings.HasPrefix(raw, "ssh://") {
		return normalizeSSHURL(raw)
	}

	return normalizeSCPStyle(raw)
}

func normalizeSSHURL(raw string) (Remote, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Remote{}, fmt.Errorf("parsing remote: %w", err)
	}
	if u.Scheme != "ssh" {
		return Remote{}, fmt.Errorf("unsupported remote scheme %q", u.Scheme)
	}

	host := strings.ToLower(u.Hostname())
	user := strings.ToLower(u.User.Username())
	path := strings.Trim(strings.TrimPrefix(u.Path, "/"), " ")
	return buildRemote(raw, user, host, path)
}

func normalizeSCPStyle(raw string) (Remote, error) {
	sep := strings.Index(raw, ":")
	if sep <= 0 {
		return Remote{}, fmt.Errorf("unsupported remote format")
	}

	hostPart := raw[:sep]
	path := strings.Trim(raw[sep+1:], " ")
	if path == "" {
		return Remote{}, fmt.Errorf("missing remote path")
	}

	user := ""
	host := hostPart
	if at := strings.Index(hostPart, "@"); at >= 0 {
		user = strings.ToLower(hostPart[:at])
		host = hostPart[at+1:]
	}

	return buildRemote(raw, user, strings.ToLower(host), path)
}

func buildRemote(raw, user, host, path string) (Remote, error) {
	path = strings.Trim(strings.TrimPrefix(path, "/"), " ")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return Remote{}, fmt.Errorf("unsupported remote path %q", path)
	}

	owner := strings.ToLower(parts[0])
	repo := strings.ToLower(parts[1])

	return Remote{
		Raw:      raw,
		User:     user,
		Host:     host,
		Owner:    owner,
		Repo:     repo,
		RouteKey: fmt.Sprintf("%s@%s:%s/%s", user, host, owner, repo),
	}, nil
}
