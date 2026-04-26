package sshrouting

import (
	"sort"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type PlanRequest struct {
	Target    Target
	Operation OperationClass
	Routes    map[string]vault.SSHRoute
	Keys      []vault.Key
	Limit     int
}

type Candidate struct {
	Fingerprint string
	Name        string
	PublicKey   string
	Score       int
	Reason      string
	Proven      bool
	Attempted   bool
	Updated     time.Time
}

type CandidatePlan struct {
	Candidates   []Candidate
	Fingerprints []string
	HadExact     bool
}

func PlanCandidatesForRequest(req PlanRequest) CandidatePlan {
	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}

	keyByFingerprint := make(map[string]vault.Key, len(req.Keys))
	for _, key := range req.Keys {
		if strings.TrimSpace(key.Fingerprint) == "" {
			continue
		}
		if _, ok := keyByFingerprint[key.Fingerprint]; !ok {
			keyByFingerprint[key.Fingerprint] = key
		}
	}

	candidates := make(map[string]Candidate, len(keyByFingerprint))
	add := func(fingerprint string, score int, reason string, proven bool, updated time.Time, attempted bool) {
		key, ok := keyByFingerprint[fingerprint]
		if !ok || fingerprint == "" {
			return
		}
		current := candidates[fingerprint]
		if current.Fingerprint == "" || score > current.Score || (score == current.Score && updated.After(current.Updated)) {
			candidates[fingerprint] = Candidate{
				Fingerprint: fingerprint,
				Name:        key.Name,
				PublicKey:   key.PublicKey,
				Score:       score,
				Reason:      reason,
				Proven:      proven,
				Attempted:   attempted || current.Attempted,
				Updated:     updated,
			}
			return
		}
		if attempted && current.Fingerprint != "" {
			current.Attempted = true
			candidates[fingerprint] = current
		}
	}

	if route, ok := req.Routes[req.Target.Canonical]; ok && route.Key != "" {
		if routeIsProof(route, req.Target.Kind) && routeProvesOperation(route.Operation, req.Operation) {
			add(route.Key, 10000, "exact", true, route.Updated, false)
		} else {
			add(route.Key, 7800, "exact_other_operation", false, route.Updated, false)
		}
	}

	for routeKey, route := range req.Routes {
		if route.Key == "" {
			continue
		}
		stored, err := ParseCanonicalTarget(routeKey)
		if err != nil || stored.Kind != req.Target.Kind {
			continue
		}
		score, reason := routeHintScore(req.Target, stored)
		if score == 0 {
			continue
		}
		if !routeProvesOperation(route.Operation, req.Operation) {
			score -= 500
			if score < 1 {
				score = 1
			}
		}
		add(route.Key, score, reason, false, route.Updated, false)
	}

	if route, ok := req.Routes[req.Target.Canonical]; ok {
		for fingerprint, attemptedAt := range route.Attempts {
			add(fingerprint, 500, "previous_attempt", false, attemptedAt, true)
		}
	}

	for _, key := range req.Keys {
		score := 1000
		updated := key.UpdatedAt
		if key.LastUsedAt != nil {
			score = 1500
			updated = *key.LastUsedAt
		}
		add(key.Fingerprint, score, "fallback", false, updated, false)
	}

	all := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		all = append(all, candidate)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Proven != all[j].Proven {
			return all[i].Proven
		}
		if all[i].Attempted != all[j].Attempted {
			return !all[i].Attempted
		}
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		if !all[i].Updated.Equal(all[j].Updated) {
			return all[i].Updated.After(all[j].Updated)
		}
		return all[i].Fingerprint < all[j].Fingerprint
	})

	plan := CandidatePlan{Candidates: all}
	for i, candidate := range all {
		if i >= limit {
			break
		}
		plan.Fingerprints = append(plan.Fingerprints, candidate.Fingerprint)
	}
	if len(all) > 0 && all[0].Reason == "exact" && all[0].Proven {
		plan.HadExact = true
	}
	return plan
}

func PlanCandidates(target Target, routes map[string]vault.SSHRoute, keys []vault.Key, limit int) CandidatePlan {
	return PlanCandidatesForRequest(PlanRequest{
		Target: target,
		Routes: routes,
		Keys:   keys,
		Limit:  limit,
	})
}

func routeHintScore(target, stored Target) (int, string) {
	if target.Kind == TargetGit {
		if strings.EqualFold(stored.Host, target.Host) &&
			strings.EqualFold(stored.User, target.User) &&
			strings.EqualFold(stored.Owner, target.Owner) {
			return 7000, "same_owner"
		}
		if strings.EqualFold(stored.Host, target.Host) && strings.EqualFold(stored.User, target.User) {
			return 5500, "same_host_user"
		}
		if strings.EqualFold(stored.Host, target.Host) {
			return 4500, "same_host"
		}
		return 0, ""
	}
	if strings.EqualFold(stored.Host, target.Host) && strings.EqualFold(stored.User, target.User) {
		return 7000, "same_host_user"
	}
	if strings.EqualFold(stored.Host, target.Host) {
		return 5500, "same_host"
	}
	return 0, ""
}

func routeProvesOperation(routeOperation string, requested OperationClass) bool {
	stored := parseOperation(routeOperation)
	if requested == "" || requested == OperationUnknown || stored == OperationUnknown {
		return true
	}
	if stored == requested {
		return true
	}
	return stored == OperationWrite && requested == OperationRead
}

func routeIsProof(route vault.SSHRoute, kind TargetKind) bool {
	if kind == TargetGit {
		return route.ProvenBy == vault.SSHRouteProofProviderProbe
	}
	return route.ProvenBy == "" || route.ProvenBy == vault.SSHRouteProofSSHAuth
}
