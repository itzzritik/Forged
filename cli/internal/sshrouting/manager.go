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
			alias := providerAlias(candidate.Provider, candidate.KeyID)
			hintPath := filepath.Join(paths.ManagedKeysDir, alias+".pub")
			state.UpsertProviderKey(ProviderKey{
				Provider:        candidate.Provider,
				KeyID:           candidate.KeyID,
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
				KeyID:        candidate.KeyID,
				IdentityFile: hintPath,
				MatchExec:    renderMatchExec(paths.HelperBinary, candidate.Provider, candidate.KeyID),
			})
		}
	}

	state.RemoveMissingProviderKeys(wantedProviderKeys)

	if err := syncHintFiles(paths.ManagedKeysDir, wantedHints); err != nil {
		return err
	}

	return os.WriteFile(paths.AdvancedConfigPath, []byte(RenderAdvancedConfig(entries)), 0o600)
}

func providerAlias(provider, keyID string) string {
	return provider + "-forged-" + aliasComponent(keyID)
}

func renderMatchExec(helperPath, provider, keyID string) string {
	cmd := shellQuote(helperPath) + " __ssh-route-match --provider " + provider + " --key-id " + shellQuote(keyID)
	return strconv.Quote(cmd)
}

func aliasComponent(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	alias := strings.Trim(b.String(), "-")
	if alias == "" {
		return "key"
	}
	return alias
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
