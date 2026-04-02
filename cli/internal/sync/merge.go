package sync

import (
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

const tombstoneTTL = 90 * 24 * time.Hour

func MergeVaults(local, remote vault.VaultData) vault.VaultData {
	merged := vault.VaultData{
		Metadata:      local.Metadata,
		KeyGeneration: max(local.KeyGeneration, remote.KeyGeneration),
	}

	merged.VersionVector = mergeVersionVectors(local.VersionVector, remote.VersionVector)
	merged.Tombstones = mergeTombstones(local.Tombstones, remote.Tombstones)

	tombstoneSet := make(map[string]bool)
	for _, t := range merged.Tombstones {
		tombstoneSet[t.KeyID] = true
	}

	keyMap := make(map[string]vault.Key)

	for _, k := range remote.Keys {
		if !tombstoneSet[k.ID] {
			keyMap[k.ID] = k
		}
	}

	for _, k := range local.Keys {
		if tombstoneSet[k.ID] {
			continue
		}
		existing, exists := keyMap[k.ID]
		if !exists {
			keyMap[k.ID] = k
		} else {
			if k.UpdatedAt.After(existing.UpdatedAt) {
				keyMap[k.ID] = k
			}
		}
	}

	merged.Keys = make([]vault.Key, 0, len(keyMap))
	for _, k := range keyMap {
		merged.Keys = append(merged.Keys, k)
	}

	return merged
}

func mergeVersionVectors(a, b map[string]int64) map[string]int64 {
	merged := make(map[string]int64)
	for k, v := range a {
		merged[k] = v
	}
	for k, v := range b {
		if v > merged[k] {
			merged[k] = v
		}
	}
	return merged
}

func mergeTombstones(a, b []vault.Tombstone) []vault.Tombstone {
	seen := make(map[string]vault.Tombstone)
	cutoff := time.Now().Add(-tombstoneTTL)

	for _, t := range a {
		if t.DeletedAt.After(cutoff) {
			seen[t.KeyID] = t
		}
	}
	for _, t := range b {
		if t.DeletedAt.After(cutoff) {
			if existing, ok := seen[t.KeyID]; !ok || t.DeletedAt.After(existing.DeletedAt) {
				seen[t.KeyID] = t
			}
		}
	}

	result := make([]vault.Tombstone, 0, len(seen))
	for _, t := range seen {
		result = append(result, t)
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
