package sync

import (
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

func TestMergeThreeWayPrefersNewerLiveRoute(t *testing.T) {
	baseTime := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-ssr"

	base := vault.VaultData{
		SSH: vault.SSHData{
			Routes: map[string]vault.SSHRoute{
				target: {Key: "SHA256:key-a", Updated: baseTime},
			},
		},
	}
	local := base
	local.SSH.Routes = map[string]vault.SSHRoute{
		target: {Key: "SHA256:key-b", Updated: baseTime.Add(2 * time.Minute)},
	}
	remote := base
	remote.SSH.Routes = map[string]vault.SSHRoute{
		target: {Updated: baseTime.Add(1 * time.Minute)},
	}

	merged := MergeThreeWay(base, local, remote, "device-1", "device-2")
	if got := merged.SSH.Routes[target].Key; got != "SHA256:key-b" {
		t.Fatalf("expected local route to win, got %q", got)
	}
}

func TestMergeThreeWayPrefersNewerClear(t *testing.T) {
	baseTime := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-ssr"

	base := vault.VaultData{
		SSH: vault.SSHData{
			Routes: map[string]vault.SSHRoute{
				target: {Key: "SHA256:key-a", Updated: baseTime},
			},
		},
	}
	local := base
	local.SSH.Routes = map[string]vault.SSHRoute{
		target: {Updated: baseTime.Add(3 * time.Minute)},
	}
	remote := base

	merged := MergeThreeWay(base, local, remote, "device-1", "device-2")
	if got := merged.SSH.Routes[target].Key; got != "" {
		t.Fatalf("expected cleared route to win, got %q", got)
	}
}
