package sshrouting

import (
	"sort"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type CandidatePlan struct {
	Fingerprints []string
	HadExact     bool
}

func PlanCandidates(target Target, routes map[string]vault.SSHRoute, keys []vault.Key, limit int) CandidatePlan {
	if limit <= 0 {
		limit = 3
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, limit)
	appendFP := func(fp string) {
		if fp == "" || len(out) >= limit {
			return
		}
		if _, ok := seen[fp]; ok {
			return
		}
		seen[fp] = struct{}{}
		out = append(out, fp)
	}

	plan := CandidatePlan{}
	if route, ok := routes[target.Canonical]; ok && route.Key != "" {
		appendFP(route.Key)
		plan.HadExact = true
	}

	type hint struct {
		rank    int
		updated int64
		key     string
	}

	var hints []hint
	for routeKey, route := range routes {
		if route.Key == "" {
			continue
		}

		stored, err := ParseCanonicalTarget(routeKey)
		if err != nil || stored.Kind != target.Kind {
			continue
		}

		switch {
		case target.Kind == TargetGit && stored.Host == target.Host && stored.User == target.User && stored.Owner == target.Owner:
			hints = append(hints, hint{rank: 1, updated: route.Updated.UnixNano(), key: route.Key})
		case stored.Host == target.Host && stored.User == target.User:
			hints = append(hints, hint{rank: 2, updated: route.Updated.UnixNano(), key: route.Key})
		}
	}

	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].rank != hints[j].rank {
			return hints[i].rank < hints[j].rank
		}
		if hints[i].updated != hints[j].updated {
			return hints[i].updated > hints[j].updated
		}
		return hints[i].key < hints[j].key
	})

	for _, hint := range hints {
		appendFP(hint.key)
	}
	for _, key := range keys {
		appendFP(key.Fingerprint)
	}

	plan.Fingerprints = out
	return plan
}
