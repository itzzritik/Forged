package sshrouting

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type ProviderCandidate struct {
	Provider    string
	MatchHost   string
	KeyID       string
	AccountSlug string
	DisplayName string
	PublicKey   string
}

var githubNamePattern = regexp.MustCompile(`(?i)^github(?:\s*[-:(]\s*|\s+)(.+?)(?:\s*\))?$`)

func DetectProviderCandidates(keys []vault.Key) []ProviderCandidate {
	out := make([]ProviderCandidate, 0, len(keys))
	for _, key := range keys {
		match := githubNamePattern.FindStringSubmatch(strings.TrimSpace(key.Name))
		if len(match) < 2 {
			continue
		}

		slug := slugifyProviderLabel(match[1])
		if slug == "" {
			continue
		}

		out = append(out, ProviderCandidate{
			Provider:    "github",
			MatchHost:   "github.com",
			KeyID:       key.ID,
			AccountSlug: slug,
			DisplayName: key.Name,
			PublicKey:   strings.TrimSpace(key.PublicKey),
		})
	}
	return out
}

func GroupProviderConflicts(candidates []ProviderCandidate) map[string][]ProviderCandidate {
	grouped := map[string][]ProviderCandidate{}
	for _, candidate := range candidates {
		key := candidate.Provider + "@" + candidate.MatchHost
		grouped[key] = append(grouped[key], candidate)
	}

	for key, group := range grouped {
		seen := map[string]struct{}{}
		for _, candidate := range group {
			seen[candidate.AccountSlug] = struct{}{}
		}
		if len(seen) < 2 {
			delete(grouped, key)
		}
	}

	return grouped
}

func slugifyProviderLabel(raw string) string {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(raw)), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	return strings.Join(parts, "-")
}
