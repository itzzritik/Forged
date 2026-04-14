package sshrouting

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "ssh-routing.json"))
	now := time.Now().UTC().Round(time.Second)

	state := State{
		Hosts: map[string]HostAffinity{
			"github.com:22": {
				KeyID:         "key-1",
				LastSuccessAt: now,
				SuccessCount:  3,
			},
		},
	}

	if err := store.Save(&state); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Hosts["github.com:22"].KeyID != "key-1" {
		t.Fatalf("unexpected host state: %#v", got.Hosts["github.com:22"])
	}
}

func TestRecordSuccessCreatesAndUpdatesAffinity(t *testing.T) {
	state := DefaultState()
	state.RecordSuccess("github.com", 22, "key-2", time.Unix(100, 0).UTC())
	state.RecordSuccess("github.com", 22, "key-2", time.Unix(200, 0).UTC())

	got := state.Hosts["github.com:22"]
	if got.KeyID != "key-2" || got.SuccessCount != 2 {
		t.Fatalf("unexpected affinity: %#v", got)
	}
}
