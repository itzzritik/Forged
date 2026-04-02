package hostmatch

import (
	"regexp"
	"strings"

	"github.com/itzzritik/forged/cli/internal/vault"
)

func Match(pattern string, host string) bool {
	if pattern == "" || host == "" {
		return false
	}

	if strings.HasPrefix(pattern, "~") {
		return matchRegex(pattern[1:], host)
	}

	if strings.Contains(pattern, "*") {
		return matchWildcard(pattern, host)
	}

	return strings.EqualFold(pattern, host)
}

func matchWildcard(pattern, host string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return false
	}

	remaining := strings.ToLower(host)
	for i, part := range parts {
		part = strings.ToLower(part)
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}

	return true
}

func matchRegex(pattern, host string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(host)
}

func ClassifyPattern(pattern string) string {
	if strings.HasPrefix(pattern, "~") {
		return "regex"
	}
	if strings.Contains(pattern, "*") {
		return "wildcard"
	}
	return "exact"
}

type scored struct {
	key   vault.Key
	score int
}

func ScoreKeys(keys []vault.Key, host string) []vault.Key {
	var results []scored
	for _, k := range keys {
		best := 0
		for _, rule := range k.HostRules {
			if Match(rule.Match, host) {
				s := scoreMatch(rule)
				if s > best {
					best = s
				}
			}
		}
		results = append(results, scored{key: k, score: best})
	}

	// Stable sort: matched keys first, then by score descending, then by last used
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if shouldSwap(results[i], results[j]) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	out := make([]vault.Key, len(results))
	for i, r := range results {
		out[i] = r.key
	}
	return out
}

func scoreMatch(rule vault.HostRule) int {
	switch rule.Type {
	case "exact":
		return 100
	case "wildcard":
		return 50
	case "regex":
		return 25
	default:
		return 50
	}
}

func shouldSwap(a, b scored) bool {
	if a.score != b.score {
		return b.score > a.score
	}
	if a.key.LastUsedAt != nil && b.key.LastUsedAt != nil {
		return b.key.LastUsedAt.After(*a.key.LastUsedAt)
	}
	if b.key.LastUsedAt != nil {
		return true
	}
	return false
}
