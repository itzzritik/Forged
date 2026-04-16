package sshrouting

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func ResolveGitTarget(cwd, branch string) (Target, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		out, err := gitOutput(cwd, "branch", "--show-current")
		if err != nil {
			return Target{}, fmt.Errorf("resolve current branch: %w", err)
		}
		branch = strings.TrimSpace(out)
	}

	remoteName := firstNonEmpty(
		mustGitConfig(cwd, "branch."+branch+".pushRemote"),
		mustGitConfig(cwd, "remote.pushDefault"),
		mustGitConfig(cwd, "branch."+branch+".remote"),
		"origin",
	)
	remoteURL := firstNonEmpty(
		mustGitConfig(cwd, "remote."+remoteName+".pushurl"),
		mustGitConfig(cwd, "remote."+remoteName+".url"),
	)
	if remoteURL == "" {
		return Target{}, fmt.Errorf("no push destination configured")
	}

	return normalizeGitRemote(remoteURL)
}

func CurrentRemote() (Remote, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Remote{}, err
	}

	target, err := ResolveGitTarget(cwd, "")
	if err != nil {
		return Remote{}, fmt.Errorf("current repo has no origin")
	}
	return remoteFromTarget(target.Canonical, target), nil
}

func normalizeGitRemote(raw string) (Target, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if trimmed == "" {
		return Target{}, fmt.Errorf("empty remote")
	}

	if strings.HasPrefix(trimmed, "ssh://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return Target{}, fmt.Errorf("parsing remote: %w", err)
		}
		if u.Scheme != "ssh" {
			return Target{}, fmt.Errorf("unsupported remote scheme %q", u.Scheme)
		}

		port, err := parsePort(u.Port(), 22)
		if err != nil {
			return Target{}, fmt.Errorf("parse port: %w", err)
		}

		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return Target{}, fmt.Errorf("git remote missing owner/repo %q", raw)
		}

		user := strings.ToLower(u.User.Username())
		host := strings.ToLower(u.Hostname())
		return Target{
			Kind:      TargetGit,
			Canonical: fmt.Sprintf("git+ssh://%s@%s:%d/%s/%s", user, host, port, parts[0], parts[1]),
			Host:      host,
			User:      user,
			Port:      port,
			Owner:     parts[0],
			Repo:      parts[1],
		}, nil
	}

	at := strings.Index(trimmed, "@")
	colon := strings.Index(trimmed, ":")
	if at < 0 || colon < 0 || colon < at {
		return Target{}, fmt.Errorf("unsupported git remote %q", raw)
	}

	user := strings.ToLower(trimmed[:at])
	host := strings.ToLower(trimmed[at+1 : colon])
	path := strings.TrimPrefix(trimmed[colon+1:], "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return Target{}, fmt.Errorf("git remote missing owner/repo %q", raw)
	}

	return Target{
		Kind:      TargetGit,
		Canonical: fmt.Sprintf("git+ssh://%s@%s:22/%s/%s", user, host, parts[0], parts[1]),
		Host:      host,
		User:      user,
		Port:      22,
		Owner:     parts[0],
		Repo:      parts[1],
	}, nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", message, err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func gitConfig(cwd string, args ...string) (string, error) {
	cmdArgs := append([]string{"config"}, args...)
	return gitOutput(cwd, cmdArgs...)
}

func mustGitConfig(cwd, key string) string {
	value, err := gitConfig(cwd, "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
