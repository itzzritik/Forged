package daemon

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCheckNPMInstalls(t *testing.T) {
	a, b := filepath.FromSlash("/a/node_modules"), filepath.FromSlash("/b/node_modules")
	cases := []struct {
		name        string
		roots       []string
		present     []string
		wantDropped []string
		installed   bool
	}{
		{"package present", []string{a}, []string{a, filepath.Join(a, "forgedssh")}, nil, true},
		{"package removed", []string{a}, []string{a}, nil, false},
		{"node version removed", []string{a}, nil, []string{a}, true},
		{"switched to the old package", []string{a, b}, []string{a, b, filepath.Join(b, "@getforged", "cli")}, nil, true},
		{"removed, other node gone", []string{a, b}, []string{a}, []string{b}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exists := func(path string) bool { return slices.Contains(tc.present, path) }
			holds := func(root string) bool {
				return exists(filepath.Join(root, "forgedssh")) || exists(filepath.Join(root, "@getforged", "cli"))
			}
			got := checkNPMInstalls(tc.roots, exists, holds)
			if !slices.Equal(got.dropped, tc.wantDropped) || got.installed != tc.installed {
				t.Fatalf("got dropped=%q installed=%v; want dropped=%q installed=%v", got.dropped, got.installed, tc.wantDropped, tc.installed)
			}
			for _, root := range got.live {
				if slices.Contains(tc.wantDropped, root) {
					t.Fatalf("dropped root %q is still live", root)
				}
			}
		})
	}
}

func TestHoldsForged(t *testing.T) {
	cases := map[string]bool{
		"forgedssh":            true,
		"@getforged/cli":       true,
		".forgedssh-1a2b3c4d":  true, // npm moved it aside while it downloads the update
		"@getforged/.cli-9f8e": true,
		"other-package":        false,
		".forgedsshx":          false,
	}
	for entry, want := range cases {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(entry)), 0o700); err != nil {
			t.Fatal(err)
		}
		if got := holdsForged(root); got != want {
			t.Errorf("holdsForged with %q = %v, want %v", entry, got, want)
		}
	}
	if holdsForged(t.TempDir()) {
		t.Error("an empty node_modules must not hold forged")
	}
}

func TestUninstallClock(t *testing.T) {
	var c uninstallClock
	t0 := time.Unix(0, 0)
	steps := []struct {
		installed bool
		at        time.Duration
		want      bool
	}{
		{false, 0, false},
		{false, 9*time.Minute + 59*time.Second, false},
		{true, 5 * time.Minute, false},
		{false, 6 * time.Minute, false},
		{false, 15*time.Minute + 59*time.Second, false},
		{false, 16 * time.Minute, true},
	}
	for i, step := range steps {
		if got := c.observe(step.installed, t0.Add(step.at)); got != step.want {
			t.Fatalf("step %d: observe(%v, +%v) = %v, want %v", i, step.installed, step.at, got, step.want)
		}
	}
}

func TestNPMSourceGuard(t *testing.T) {
	roots := []string{filepath.FromSlash("/g/node_modules")}
	cache := filepath.FromSlash("/home/r/.cache/forged")
	cases := map[string]bool{
		filepath.FromSlash("/g/node_modules/forgedssh/bin/forged"): true,
		filepath.Join(cache, "0.1.42", "forged"):                    true,
		filepath.FromSlash("/Users/r/forged/bin/forged"):           false,
		"": false,
	}
	for source, want := range cases {
		if got := npmSourceGuard(source, roots, cache); got != want {
			t.Errorf("npmSourceGuard(%q) = %v, want %v", source, got, want)
		}
	}
}
