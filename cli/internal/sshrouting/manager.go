package sshrouting

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type ManagedPaths struct {
	AdvancedConfigPath string
	ManagedKeysDir     string
	HelperBinary       string
}

func RefreshAdvancedProviderRouting(paths ManagedPaths, state *State, keys []vault.Key) error {
	candidates := DetectProviderCandidates(keys)
	conflicts := GroupProviderConflicts(candidates)

	if err := os.MkdirAll(paths.ManagedKeysDir, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.AdvancedConfigPath), 0o700); err != nil {
		return err
	}

	wantedHints := map[string]string{}
	wantedProviderKeys := map[string]struct{}{}
	entries := make([]ProviderRouteEntry, 0)
	now := time.Now().UTC()

	for _, group := range conflicts {
		for _, candidate := range group {
			alias := providerAlias(candidate.Provider, candidate.AccountSlug)
			hintPath := filepath.Join(paths.ManagedKeysDir, alias+".pub")
			state.UpsertProviderIdentity(ProviderIdentity{
				Provider:        candidate.Provider,
				KeyID:           candidate.KeyID,
				AccountSlug:     candidate.AccountSlug,
				MatchHost:       candidate.MatchHost,
				Alias:           alias,
				HintPath:        hintPath,
				LastRefreshedAt: now,
			})
			wantedProviderKeys[candidate.KeyID] = struct{}{}
			wantedHints[hintPath] = candidate.PublicKey + "\n"
			entries = append(entries, ProviderRouteEntry{
				MatchHost:    candidate.MatchHost,
				Provider:     candidate.Provider,
				AccountSlug:  candidate.AccountSlug,
				IdentityFile: hintPath,
				MatchExec:    renderMatchExec(paths.HelperBinary, candidate.Provider, candidate.AccountSlug),
			})
		}
	}

	state.RemoveMissingProviderIdentities(wantedProviderKeys)

	if err := syncHintFiles(paths.ManagedKeysDir, wantedHints); err != nil {
		return err
	}

	return os.WriteFile(paths.AdvancedConfigPath, []byte(RenderAdvancedConfig(entries)), 0o600)
}

func providerAlias(provider, slug string) string {
	return provider + "-forged-" + slug
}

func renderMatchExec(helperPath, provider, account string) string {
	cmd := shellQuote(helperPath) + " __ssh-route-match --provider " + provider + " --account " + account
	return strconv.Quote(cmd)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\"'\"'`) + "'"
}

func syncHintFiles(dir string, wanted map[string]string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	existing, err := filepath.Glob(filepath.Join(dir, "*.pub"))
	if err != nil {
		return err
	}
	for _, path := range existing {
		if _, ok := wanted[path]; !ok {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	for path, content := range wanted {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}

	return nil
}
