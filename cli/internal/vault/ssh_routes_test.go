package vault

import (
	"path/filepath"
	"testing"
	"time"
)

func newRouteKeyStore(t *testing.T) *KeyStore {
	t.Helper()

	v, err := Create(filepath.Join(t.TempDir(), "vault.forged"), []byte("correct horse battery staple"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Cleanup(v.Close)

	v.Data.Metadata.DeviceID = "device-1"
	return NewKeyStore(v)
}

func TestRecordSSHRouteStoresFingerprintAndUpdated(t *testing.T) {
	ks := newRouteKeyStore(t)
	now := time.Date(2026, 4, 16, 12, 34, 56, 0, time.UTC)
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-ssr"

	if err := ks.RecordSSHRoute(target, "SHA256:key-a", now); err != nil {
		t.Fatalf("record route: %v", err)
	}

	got, ok := ks.SSHRoute(target)
	if !ok {
		t.Fatalf("expected route %q to exist", target)
	}
	if got.Key != "SHA256:key-a" {
		t.Fatalf("expected key SHA256:key-a, got %q", got.Key)
	}
	if !got.Updated.Equal(now) {
		t.Fatalf("expected updated %s, got %s", now, got.Updated)
	}
}

func TestClearSSHRouteRemovesKeyButKeepsUpdated(t *testing.T) {
	ks := newRouteKeyStore(t)
	target := "git+ssh://git@github.com:22/AdeptMind/dlp-ssr"
	first := time.Date(2026, 4, 16, 12, 34, 56, 0, time.UTC)
	second := first.Add(5 * time.Minute)

	if err := ks.RecordSSHRoute(target, "SHA256:key-a", first); err != nil {
		t.Fatalf("record route: %v", err)
	}
	if err := ks.ClearSSHRoute(target, second); err != nil {
		t.Fatalf("clear route: %v", err)
	}

	got, ok := ks.SSHRoute(target)
	if !ok {
		t.Fatalf("expected cleared route entry to remain")
	}
	if got.Key != "" {
		t.Fatalf("expected cleared route to have empty key, got %q", got.Key)
	}
	if !got.Updated.Equal(second) {
		t.Fatalf("expected updated %s, got %s", second, got.Updated)
	}
}
