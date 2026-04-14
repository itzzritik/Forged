package sshrouting

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

const routeProbeTTL = 5 * time.Second

type ProviderCandidate struct {
	Provider  string
	MatchHost string
	KeyID     string
	Alias     string
	PublicKey string
}

type MatchRuntime struct {
	StatePath      string
	ManagedKeysDir string
	AgentSocket    string
}

func DetectProviderCandidates(keys []vault.Key) []ProviderCandidate {
	if len(keys) < 2 {
		return nil
	}

	out := make([]ProviderCandidate, 0, len(keys))
	for _, key := range keys {
		publicKey := strings.TrimSpace(key.PublicKey)
		if publicKey == "" {
			continue
		}

		out = append(out, ProviderCandidate{
			Provider:  "github",
			MatchHost: "github.com",
			KeyID:     key.ID,
			Alias:     providerAlias("github", key.ID),
			PublicKey: publicKey,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].KeyID < out[j].KeyID
	})
	return out
}

func GroupProviderConflicts(candidates []ProviderCandidate) map[string][]ProviderCandidate {
	grouped := map[string][]ProviderCandidate{}
	for _, candidate := range candidates {
		key := candidate.Provider + "@" + candidate.MatchHost
		grouped[key] = append(grouped[key], candidate)
	}

	for key, group := range grouped {
		if len(group) < 2 {
			delete(grouped, key)
		}
	}

	return grouped
}

func MatchProviderKey(cwd, provider, keyID string, runtime MatchRuntime) bool {
	ctx, ok := DetectRepoContext(cwd)
	if !ok {
		return false
	}

	switch provider {
	case "github":
		return matchGitHubKey(ctx, keyID, runtime)
	default:
		return false
	}
}

func matchGitHubKey(ctx RepoContext, keyID string, runtime MatchRuntime) bool {
	if !strings.EqualFold(ctx.Host, "github.com") {
		return false
	}

	store := NewStore(runtime.StatePath)
	state, err := store.Load()
	if err != nil {
		return false
	}

	repoKey := providerRepoKey("github", ctx.Host, ctx.Owner, ctx.Repo)
	candidates := githubCandidates(state)
	if len(candidates) < 2 {
		return false
	}

	if route, ok := state.RepoRoutes[repoKey]; ok && route.KeyID != "" {
		if time.Since(route.LastVerifiedAt) <= routeProbeTTL {
			return route.KeyID == keyID
		}
		if cached := candidateByID(candidates, route.KeyID); cached != nil {
			ok, err := probeGitHubRepoAccess(ctx.RemoteURL, runtime.AgentSocket, hintPathFor(runtime, *cached), ctx.Host)
			if err == nil && ok {
				route.LastVerifiedAt = time.Now().UTC()
				state.RepoRoutes[repoKey] = route
				_ = store.Save(state)
				return route.KeyID == keyID
			}
		}
	}

	resolvedKeyID, ok := resolveGitHubRepoKey(ctx, runtime, candidates)
	if !ok {
		if state.RepoRoutes != nil {
			delete(state.RepoRoutes, repoKey)
			_ = store.Save(state)
		}
		return false
	}

	state.UpsertRepoRoute(RepoRoute{
		Provider:       "github",
		MatchHost:      "github.com",
		RepoKey:        repoKey,
		KeyID:          resolvedKeyID,
		LastVerifiedAt: time.Now().UTC(),
	})
	if err := store.Save(state); err != nil {
		return false
	}
	return resolvedKeyID == keyID
}

func githubCandidates(state *State) []ProviderKey {
	if state == nil || len(state.ProviderKeys) == 0 {
		return nil
	}

	keys := make([]ProviderKey, 0, len(state.ProviderKeys))
	for _, key := range state.ProviderKeys {
		if key.Provider == "github" && strings.EqualFold(key.MatchHost, "github.com") {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].KeyID < keys[j].KeyID
	})
	return keys
}

func providerRepoKey(provider, host, owner, repo string) string {
	return strings.ToLower(provider) + ":" + strings.ToLower(host) + ":" + strings.ToLower(owner) + "/" + strings.ToLower(repo)
}

func candidateByID(candidates []ProviderKey, keyID string) *ProviderKey {
	for i := range candidates {
		if candidates[i].KeyID == keyID {
			return &candidates[i]
		}
	}
	return nil
}

func hintPathFor(runtime MatchRuntime, candidate ProviderKey) string {
	if candidate.HintPath != "" {
		return candidate.HintPath
	}
	return filepath.Join(runtime.ManagedKeysDir, candidate.Alias+".pub")
}

func resolveGitHubRepoKey(ctx RepoContext, runtime MatchRuntime, candidates []ProviderKey) (string, bool) {
	for _, candidate := range candidates {
		ok, err := probeGitHubRepoAccess(ctx.RemoteURL, runtime.AgentSocket, hintPathFor(runtime, candidate), ctx.Host)
		if err == nil && ok {
			return candidate.KeyID, true
		}
	}
	return "", false
}

func probeGitHubRepoAccess(remoteURL, agentSocket, identityFile, host string) (bool, error) {
	if remoteURL == "" || agentSocket == "" || identityFile == "" || host == "" {
		return false, fmt.Errorf("missing probe inputs")
	}

	knownHostsFile, err := buildKnownHosts(host)
	if err != nil {
		return false, err
	}
	defer os.Remove(knownHostsFile)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	sshCommand := fmt.Sprintf(
		"ssh -F /dev/null -o BatchMode=yes -o PreferredAuthentications=publickey -o IdentitiesOnly=yes -o IdentityAgent=%s -o IdentityFile=%s -o UserKnownHostsFile=%s -o StrictHostKeyChecking=yes",
		shellEscape(agentSocket),
		shellEscape(identityFile),
		shellEscape(knownHostsFile),
	)

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", remoteURL)
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand)
	var stderr bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return false, fmt.Errorf(strings.TrimSpace(stderr.String()))
}

func buildKnownHosts(host string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh-keyscan", "-T", "5", host)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return "", fmt.Errorf("no host keys returned for %s", host)
	}

	file, err := os.CreateTemp("", "forged-known-hosts-*")
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.Write(out); err != nil {
		return "", err
	}
	if err := file.Chmod(0o600); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\"'\"'`) + "'"
}
