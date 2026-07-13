package sensitiveauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
)

// tmpPaths returns paths rooted at an empty temp dir, so config.Load falls back
// to the default 7-day Master Password Interval.
func tmpPaths(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	p := config.Paths{ConfigDir: dir, DataDir: dir}
	if err := os.MkdirAll(p.AuthDir(), 0o700); err != nil {
		t.Fatalf("mkdir auth: %v", err)
	}
	return p
}

func TestEnrollmentExpiredSlidingWindow(t *testing.T) {
	p := tmpPaths(t)
	now := time.Now().UTC()
	day := 24 * time.Hour

	cases := []struct {
		name string
		e    LocalEnrollment
		want bool
	}{
		{"fresh", LocalEnrollment{CreatedAt: now, LastUsedAt: now}, false},
		{"active slide (used 1h ago, enrolled 30d ago)", LocalEnrollment{CreatedAt: now.Add(-30 * day), LastUsedAt: now.Add(-time.Hour)}, false},
		{"inactive 8d", LocalEnrollment{CreatedAt: now.Add(-40 * day), LastUsedAt: now.Add(-8 * day)}, true},
		{"hard cap (used 1h ago but enrolled 91d ago)", LocalEnrollment{CreatedAt: now.Add(-91 * day), LastUsedAt: now.Add(-time.Hour)}, true},
		{"legacy blob, created 3d ago, no LastUsedAt", LocalEnrollment{CreatedAt: now.Add(-3 * day)}, false},
		{"legacy blob, created 8d ago, no LastUsedAt", LocalEnrollment{CreatedAt: now.Add(-8 * day)}, true},
		{"headless never expires", LocalEnrollment{TrustMode: LocalEnrollmentTrustHeadlessFile, CreatedAt: now.Add(-999 * day)}, false},
		{"nil-ish (zero times)", LocalEnrollment{}, false},
	}
	for _, tc := range cases {
		e := tc.e
		if got := enrollmentExpired(p, &e); got != tc.want {
			t.Errorf("%s: enrollmentExpired = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLocalEnrollmentUsable(t *testing.T) {
	p := tmpPaths(t)
	now := time.Now().UTC()
	writeInstall := func(id string) {
		if err := os.WriteFile(p.InstallIDFile(), []byte(id+"\n"), 0o600); err != nil {
			t.Fatalf("write install id: %v", err)
		}
	}
	writeBlob := func(e LocalEnrollment) {
		if err := WriteLocalEnrollment(p.LocalUnlockBlobFile(), e); err != nil {
			t.Fatalf("write blob: %v", err)
		}
	}

	// No blob -> not usable.
	if LocalEnrollmentUsable(p) {
		t.Fatal("no blob should not be usable")
	}

	writeInstall("install-A")

	// Valid, non-expired, matching install -> usable.
	writeBlob(LocalEnrollment{InstallID: "install-A", CreatedAt: now, LastUsedAt: now})
	if !LocalEnrollmentUsable(p) {
		t.Error("fresh matching blob should be usable")
	}

	// Expired -> not usable.
	writeBlob(LocalEnrollment{InstallID: "install-A", CreatedAt: now.Add(-40 * 24 * time.Hour), LastUsedAt: now.Add(-8 * 24 * time.Hour)})
	if LocalEnrollmentUsable(p) {
		t.Error("expired blob should not be usable")
	}

	// Install ID mismatch -> not usable.
	writeBlob(LocalEnrollment{InstallID: "install-B", CreatedAt: now, LastUsedAt: now})
	if LocalEnrollmentUsable(p) {
		t.Error("install-id mismatch should not be usable")
	}
}

func TestRenewLocalEnrollmentUsage(t *testing.T) {
	p := tmpPaths(t)
	now := time.Now().UTC()
	read := func() *LocalEnrollment {
		e, err := ReadLocalEnrollment(p.LocalUnlockBlobFile())
		if err != nil {
			t.Fatalf("read blob: %v", err)
		}
		return e
	}

	// Stale LastUsedAt gets slid forward.
	old := now.Add(-2 * time.Hour)
	if err := WriteLocalEnrollment(p.LocalUnlockBlobFile(), LocalEnrollment{InstallID: "x", CreatedAt: now.Add(-3 * 24 * time.Hour), LastUsedAt: old}); err != nil {
		t.Fatal(err)
	}
	RenewLocalEnrollmentUsage(p)
	if got := read().LastUsedAt; !got.After(old) {
		t.Errorf("expected LastUsedAt slid forward, got %v (was %v)", got, old)
	}

	// Within the 1h throttle -> unchanged.
	recent := now.Add(-30 * time.Minute)
	if err := WriteLocalEnrollment(p.LocalUnlockBlobFile(), LocalEnrollment{InstallID: "x", CreatedAt: now, LastUsedAt: recent}); err != nil {
		t.Fatal(err)
	}
	RenewLocalEnrollmentUsage(p)
	if got := read().LastUsedAt; !got.Equal(recent) {
		t.Errorf("throttle: expected LastUsedAt unchanged %v, got %v", recent, got)
	}

	// Headless -> never touched.
	if err := WriteLocalEnrollment(p.LocalUnlockBlobFile(), LocalEnrollment{TrustMode: LocalEnrollmentTrustHeadlessFile, LastUsedAt: old}); err != nil {
		t.Fatal(err)
	}
	RenewLocalEnrollmentUsage(p)
	if got := read().LastUsedAt; !got.Equal(old) {
		t.Errorf("headless: expected LastUsedAt unchanged %v, got %v", old, got)
	}
}

// sanity: the blob path resolves under the temp auth dir.
func TestBlobPath(t *testing.T) {
	p := tmpPaths(t)
	if want := filepath.Join(p.AuthDir(), "local-unlock.json"); p.LocalUnlockBlobFile() != want {
		t.Fatalf("blob path = %q, want %q", p.LocalUnlockBlobFile(), want)
	}
}
