package sshrouting

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func ResolveGitTarget(cwd, branch string) (Target, error) {
	return ResolveGitTargetForOperationContext(context.Background(), cwd, branch, OperationUnknown)
}

func ResolveGitTargetForOperation(cwd, branch string, operation OperationClass) (Target, error) {
	return ResolveGitTargetForOperationContext(context.Background(), cwd, branch, operation)
}

func ResolveGitTargetForOperationContext(ctx context.Context, cwd, branch string, operation OperationClass) (Target, error) {
	if err := ctx.Err(); err != nil {
		return Target{}, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		out, err := gitOutputContext(ctx, cwd, "branch", "--show-current")
		if err != nil {
			return Target{}, fmt.Errorf("Resolving current branch: %w", err)
		}
		branch = strings.TrimSpace(out)
	}

	remoteName := firstNonEmpty(
		mustGitConfigContext(ctx, cwd, "branch."+branch+".pushRemote"),
		mustGitConfigContext(ctx, cwd, "remote.pushDefault"),
		mustGitConfigContext(ctx, cwd, "branch."+branch+".remote"),
		"origin",
	)
	var remoteURL string
	if operation == OperationWrite {
		remoteURL = firstNonEmpty(
			mustGitConfigContext(ctx, cwd, "remote."+remoteName+".pushurl"),
			mustGitConfigContext(ctx, cwd, "remote."+remoteName+".url"),
		)
	} else {
		remoteURL = firstNonEmpty(
			mustGitConfigContext(ctx, cwd, "remote."+remoteName+".url"),
			mustGitConfigContext(ctx, cwd, "remote."+remoteName+".pushurl"),
		)
	}
	if err := ctx.Err(); err != nil {
		return Target{}, err
	}
	if remoteURL == "" {
		return Target{}, fmt.Errorf("No push destination configured")
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
		return Remote{}, fmt.Errorf("Current repo has no origin")
	}
	return remoteFromTarget(target.Canonical, target), nil
}

func normalizeGitRemote(raw string) (Target, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if trimmed == "" {
		return Target{}, fmt.Errorf("Empty remote")
	}

	if strings.HasPrefix(trimmed, "ssh://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return Target{}, fmt.Errorf("Parsing remote: %w", err)
		}
		if u.Scheme != "ssh" {
			return Target{}, fmt.Errorf("Unsupported remote scheme %q", u.Scheme)
		}

		port, err := parsePort(u.Port(), 22)
		if err != nil {
			return Target{}, fmt.Errorf("Parsing port: %w", err)
		}

		path := normalizeRepoPath(u.Path)
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return Target{}, fmt.Errorf("Git remote is missing owner/repo %q", raw)
		}

		user := strings.ToLower(u.User.Username())
		host := strings.ToLower(u.Hostname())
		repo := strings.Join(parts[1:], "/")
		return Target{
			Kind:      TargetGit,
			Canonical: fmt.Sprintf("git+ssh://%s@%s:%d/%s/%s", user, host, port, parts[0], repo),
			Host:      host,
			User:      user,
			Port:      port,
			Owner:     parts[0],
			Repo:      repo,
		}, nil
	}

	at := strings.Index(trimmed, "@")
	colon := strings.Index(trimmed, ":")
	if at < 0 || colon < 0 || colon < at {
		return Target{}, fmt.Errorf("Unsupported git remote %q", raw)
	}

	user := strings.ToLower(trimmed[:at])
	host := strings.ToLower(trimmed[at+1 : colon])
	path := normalizeRepoPath(trimmed[colon+1:])
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return Target{}, fmt.Errorf("Git remote is missing owner/repo %q", raw)
	}
	repo := strings.Join(parts[1:], "/")

	return Target{
		Kind:      TargetGit,
		Canonical: fmt.Sprintf("git+ssh://%s@%s:22/%s/%s", user, host, parts[0], repo),
		Host:      host,
		User:      user,
		Port:      22,
		Owner:     parts[0],
		Repo:      repo,
	}, nil
}

func targetFromRepoPath(input PrepareInput, path string) (Target, error) {
	host := strings.TrimSpace(input.Host)
	user := strings.TrimSpace(input.User)
	if host == "" || user == "" {
		return Target{}, fmt.Errorf("Git target requires host and user")
	}
	port, err := parsePort(input.Port, 22)
	if err != nil {
		return Target{}, fmt.Errorf("Parsing SSH port: %w", err)
	}

	normalized := normalizeRepoPath(path)
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 {
		return Target{}, fmt.Errorf("Git command is missing owner/repo: %q", path)
	}
	repo := strings.Join(parts[1:], "/")
	host = strings.ToLower(host)
	user = strings.ToLower(user)
	return Target{
		Kind:         TargetGit,
		Canonical:    fmt.Sprintf("git+ssh://%s@%s:%d/%s/%s", user, host, port, parts[0], repo),
		Host:         host,
		OriginalHost: strings.ToLower(strings.TrimSpace(input.OriginalHost)),
		User:         user,
		Port:         port,
		Owner:        parts[0],
		Repo:         repo,
	}, nil
}

func normalizeRepoPath(path string) string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.TrimPrefix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	return strings.Trim(trimmed, "/")
}

func gitOutputContext(ctx context.Context, cwd string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", message, err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func mustGitConfigContext(ctx context.Context, cwd, key string) string {
	value, err := gitOutputContext(ctx, cwd, "config", "--get", key)
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
