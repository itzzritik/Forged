package sync

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func key(id, name, fp string, updated int) vault.Key {
	return vault.Key{ID: id, Name: name, Fingerprint: fp, UpdatedAt: t0.Add(time.Duration(updated) * time.Hour), Version: 1}
}

func data(keys ...vault.Key) vault.VaultData { return vault.VaultData{Keys: keys} }

func byID(keys []vault.Key) map[string]vault.Key {
	m := map[string]vault.Key{}
	for _, k := range keys {
		m[k.ID] = k
	}
	return m
}

func TestMergeThreeWay(t *testing.T) {
	base := key("a", "work", "fp-a", 0)
	base.Comment = "old"

	t.Run("each side's own field edit survives", func(t *testing.T) {
		local, remote := base, base
		local.Name, local.UpdatedAt = "work-renamed", t0.Add(2*time.Hour)
		remote.Comment, remote.UpdatedAt = "new comment", t0.Add(time.Hour)
		got := byID(MergeThreeWay(data(base), data(local), data(remote), "A", "B").Keys)["a"]
		if got.Name != "work-renamed" || got.Comment != "new comment" {
			t.Fatalf("got name=%q comment=%q", got.Name, got.Comment)
		}
	})

	t.Run("a delete without tombstone beats an untouched copy", func(t *testing.T) {
		got := MergeThreeWay(data(base), data(), data(base), "A", "B").Keys
		if len(got) != 0 {
			t.Fatalf("deleted key came back: %+v", got)
		}
	})

	t.Run("an old tombstone still deletes", func(t *testing.T) {
		remote := data(base)
		remote.Tombstones = []vault.Tombstone{{KeyID: "a", DeletedAt: t0.AddDate(-2, 0, 0)}}
		if got := MergeThreeWay(vault.VaultData{}, data(base), remote, "A", "B").Keys; len(got) != 0 {
			t.Fatalf("tombstoned key kept: %+v", got)
		}
	})

	t.Run("new keys from both sides are kept", func(t *testing.T) {
		got := MergeThreeWay(data(base), data(base, key("b", "b", "fp-b", 1)), data(base, key("c", "c", "fp-c", 1)), "A", "B").Keys
		if len(got) != 3 {
			t.Fatalf("got %d keys, want 3", len(got))
		}
	})

	t.Run("one key added on two devices merges to the smallest ID", func(t *testing.T) {
		l, r := key("z", "laptop", "fp-same", 1), key("m", "desktop", "fp-same", 1)
		l.GitSigning = true
		got := MergeThreeWay(vault.VaultData{}, data(l), data(r), "A", "B").Keys
		if len(got) != 1 || got[0].ID != "m" || !got[0].GitSigning {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("unknown fields survive", func(t *testing.T) {
		local := data(base)
		local.Extra = vault.Extra{"future_top": json.RawMessage(`1`)}
		remote := data(base)
		remote.Keys[0].Extra = vault.Extra{"future_key": json.RawMessage(`"x"`)}
		got := MergeThreeWay(data(base), local, remote, "A", "B")
		if got.Extra["future_top"] == nil || got.Keys[0].Extra["future_key"] == nil {
			t.Fatalf("lost extras: top=%v key=%v", got.Extra, got.Keys[0].Extra)
		}
	})
}

func FuzzMergeThreeWay(f *testing.F) {
	seed := func(v vault.VaultData) string { b, _ := json.Marshal(v); return string(b) }
	k1, k2 := key("a", "work", "fp-a", 0), key("b", "work", "fp-a", 1)
	k2.GitSigning = true
	withTomb := data(k1)
	withTomb.Tombstones = []vault.Tombstone{{KeyID: "b"}}
	f.Add(seed(data(k1)), seed(data(k1, k2)), seed(withTomb))
	f.Add("{}", `{"keys":[{"id":"x","name":"n"},{"id":"y","name":"n"},{"id":"z","name":"n (2)"}]}`, `{"keys":null}`)

	f.Fuzz(func(t *testing.T, baseJSON, localJSON, remoteJSON string) {
		var base, local, remote vault.VaultData
		if json.Unmarshal([]byte(baseJSON), &base) != nil || json.Unmarshal([]byte(localJSON), &local) != nil || json.Unmarshal([]byte(remoteJSON), &remote) != nil {
			return
		}
		merged := MergeThreeWay(base, local, remote, "A", "B")

		tombstoned := map[string]bool{}
		for _, tomb := range merged.Tombstones {
			tombstoned[tomb.KeyID] = true
		}
		ids, fps, names, signing := map[string]bool{}, map[string]bool{}, map[string]bool{}, 0
		for _, k := range merged.Keys {
			if ids[k.ID] || tombstoned[k.ID] || names[k.Name] || (k.Fingerprint != "" && fps[k.Fingerprint]) {
				t.Fatalf("duplicate or tombstoned key %+v in %+v", k, merged.Keys)
			}
			ids[k.ID], names[k.Name], fps[k.Fingerprint] = true, true, k.Fingerprint != ""
			if k.GitSigning {
				signing++
			}
		}
		if signing > 1 {
			t.Fatalf("%d git signing keys", signing)
		}
		if !sort.SliceIsSorted(merged.Keys, func(i, j int) bool { return merged.Keys[i].Name < merged.Keys[j].Name }) {
			t.Fatalf("keys not sorted: %+v", merged.Keys)
		}
	})
}
