package sshrouting

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
)

func newRoutingService(t *testing.T) (*Service, *vault.KeyStore) {
	t.Helper()

	v, err := vault.Create(filepath.Join(t.TempDir(), "vault.forged"), []byte("correct horse battery staple"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Cleanup(v.Close)
	v.Data.Metadata.DeviceID = "device-1"

	ks := vault.NewKeyStore(v)
	svc := NewService(config.Paths{}, ks)
	svc.now = func() time.Time {
		return time.Date(2026, 4, 16, 12, 34, 56, 0, time.UTC)
	}
	return svc, ks
}

func TestPrepareRegistersAttemptWithCandidatePlan(t *testing.T) {
	svc, ks := newRoutingService(t)
	if err := ks.RecordSSHRoute("git+ssh://git@github.com:22/AdeptMind/dlp-ssr", "SHA256:key-a", svc.now()); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	err := svc.Prepare(PrepareRequest{
		Attempt:   "attempt-1",
		ClientPID: 4242,
		CWD:       t.TempDir(),
		Host:      "github.com",
		User:      "git",
		Port:      "22",
		Branch:    "",
		Target: Target{
			Kind:      TargetGit,
			Canonical: "git+ssh://git@github.com:22/AdeptMind/dlp-web",
			Host:      "github.com",
			User:      "git",
			Port:      22,
			Owner:     "AdeptMind",
			Repo:      "dlp-web",
		},
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	attempt, ok := svc.AttemptByPID(4242)
	if !ok {
		t.Fatalf("expected attempt for client pid 4242")
	}
	if len(attempt.Candidates) == 0 || attempt.Candidates[0] != "SHA256:key-a" {
		t.Fatalf("unexpected candidates: %#v", attempt.Candidates)
	}
}

func TestSuccessWritesExactWinningRoute(t *testing.T) {
	svc, _ := newRoutingService(t)
	svc.attempts["attempt-1"] = Attempt{
		Token:      "attempt-1",
		ClientPID:  4242,
		Target:     Target{Canonical: "git+ssh://git@github.com:22/AdeptMind/dlp-web"},
		Candidates: []string{"SHA256:key-a", "SHA256:key-b"},
		LastKey:    "SHA256:key-b",
	}

	if err := svc.Success("attempt-1"); err != nil {
		t.Fatalf("success: %v", err)
	}
	route, ok := svc.keyStore.SSHRoute("git+ssh://git@github.com:22/AdeptMind/dlp-web")
	if !ok || route.Key != "SHA256:key-b" {
		t.Fatalf("unexpected route after success: %#v", route)
	}
}

func TestExpireClearsStaleExactRouteAfterSignatureUse(t *testing.T) {
	svc, ks := newRoutingService(t)
	now := svc.now()
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-ssr"
	if err := ks.RecordSSHRoute(target, "SHA256:key-a", now.Add(-10*time.Minute)); err != nil {
		t.Fatalf("seed route: %v", err)
	}
	svc.attempts["attempt-1"] = Attempt{
		Token:      "attempt-1",
		ClientPID:  4242,
		Target:     Target{Canonical: target},
		HadExact:   true,
		Candidates: []string{"SHA256:key-a"},
		LastKey:    "SHA256:key-a",
		Created:    now.Add(-2 * time.Minute),
	}
	svc.clientAttempt[4242] = "attempt-1"

	svc.ExpireBefore(now.Add(-30 * time.Second))

	route, ok := ks.SSHRoute(target)
	if !ok || route.Key != "" {
		t.Fatalf("expected cleared route after expiry, got %#v", route)
	}
}

func TestPrepareUsesOrgHintWithoutTreatingOwnerAsAuthenticatedUser(t *testing.T) {
	svc, ks := newRoutingService(t)
	now := svc.now()
	if err := ks.RecordSSHRoute("git+ssh://git@github.com:22/AdeptMind/dlp-ssr", "SHA256:work-key", now); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	err := svc.Prepare(PrepareRequest{
		Attempt:   "attempt-2",
		ClientPID: 5252,
		Target: Target{
			Kind:      TargetGit,
			Canonical: "git+ssh://git@github.com:22/AdeptMind/dlp-web",
			Host:      "github.com",
			User:      "git",
			Port:      22,
			Owner:     "AdeptMind",
			Repo:      "dlp-web",
		},
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	attempt, ok := svc.AttemptByPID(5252)
	if !ok || len(attempt.Candidates) == 0 || attempt.Candidates[0] != "SHA256:work-key" {
		t.Fatalf("expected org hint to win first, got %#v", attempt)
	}
}

func TestSuccessCanRepopulateClearedRoute(t *testing.T) {
	svc, ks := newRoutingService(t)
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-web"
	if err := ks.ClearSSHRoute(target, svc.now()); err != nil {
		t.Fatalf("clear route: %v", err)
	}

	svc.attempts["attempt-3"] = Attempt{
		Token:      "attempt-3",
		ClientPID:  5353,
		Target:     Target{Canonical: target},
		LastKey:    "SHA256:new-key",
		Candidates: []string{"SHA256:new-key"},
	}
	svc.clientAttempt[5353] = "attempt-3"

	if err := svc.Success("attempt-3"); err != nil {
		t.Fatalf("success: %v", err)
	}
	route, ok := ks.SSHRoute(target)
	if !ok || route.Key != "SHA256:new-key" {
		t.Fatalf("expected repopulated route, got %#v", route)
	}
}
