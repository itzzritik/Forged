package sshrouting

import (
	"testing"

	"github.com/itzzritik/forged/cli/internal/vault"
)

func TestLegacyHostRulesBecomeAdvancedHints(t *testing.T) {
	keys := []vault.Key{
		{
			ID:   "k1",
			Name: "Github (Work)",
			HostRules: []vault.HostRule{
				{Match: "github.com", Type: "exact"},
			},
			PublicKey: "ssh-ed25519 AAAA",
		},
	}

	hints := LegacyHints(keys)
	if len(hints) != 1 {
		t.Fatalf("expected one hint, got %#v", hints)
	}
	if hints[0].Host != "github.com" || hints[0].KeyID != "k1" {
		t.Fatalf("unexpected hint: %#v", hints[0])
	}
}
