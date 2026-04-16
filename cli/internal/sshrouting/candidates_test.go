package sshrouting

import (
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

func TestPlanCandidatesPrefersExactThenOwnerThenHost(t *testing.T) {
	keys := []vault.Key{
		{Fingerprint: "SHA256:key-a"},
		{Fingerprint: "SHA256:key-b"},
		{Fingerprint: "SHA256:key-c"},
	}
	routes := map[string]vault.SSHRoute{
		"git+ssh://git@github.com:22/AdeptMind/dlp-ssr": {
			Key:     "SHA256:key-a",
			Updated: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		},
		"git+ssh://git@github.com:22/OtherOrg/app": {
			Key:     "SHA256:key-b",
			Updated: time.Date(2026, 4, 16, 12, 1, 0, 0, time.UTC),
		},
		"ssh://ubuntu@144.24.124.129:22": {
			Key:     "SHA256:key-c",
			Updated: time.Date(2026, 4, 16, 12, 2, 0, 0, time.UTC),
		},
	}
	target := Target{
		Kind:      TargetGit,
		Canonical: "git+ssh://git@github.com:22/AdeptMind/dlp-web",
		Host:      "github.com",
		User:      "git",
		Owner:     "AdeptMind",
		Repo:      "dlp-web",
	}

	plan := PlanCandidates(target, routes, keys, 3)
	want := []string{"SHA256:key-a", "SHA256:key-b", "SHA256:key-c"}
	if len(plan.Fingerprints) != len(want) {
		t.Fatalf("expected %d fingerprints, got %d", len(want), len(plan.Fingerprints))
	}
	for i, wantFP := range want {
		if plan.Fingerprints[i] != wantFP {
			t.Fatalf("fingerprint %d: want %q, got %q", i, wantFP, plan.Fingerprints[i])
		}
	}
	if plan.HadExact {
		t.Fatalf("did not expect exact hit for new repo")
	}
}

func TestPlanCandidatesIgnoresClearedRoutes(t *testing.T) {
	keys := []vault.Key{{Fingerprint: "SHA256:key-a"}, {Fingerprint: "SHA256:key-b"}}
	routes := map[string]vault.SSHRoute{
		"git+ssh://git@github.com:22/AdeptMind/dlp-ssr": {
			Updated: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		},
	}
	target := Target{
		Kind:      TargetGit,
		Canonical: "git+ssh://git@github.com:22/AdeptMind/dlp-web",
		Host:      "github.com",
		User:      "git",
		Owner:     "AdeptMind",
		Repo:      "dlp-web",
	}

	plan := PlanCandidates(target, routes, keys, 2)
	if plan.Fingerprints[0] != "SHA256:key-a" {
		t.Fatalf("expected stable fallback ordering, got %#v", plan.Fingerprints)
	}
}
