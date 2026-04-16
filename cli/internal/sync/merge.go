package sync

import (
	"sort"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/vault"
)

const tombstoneTTL = 90 * 24 * time.Hour

func MergeVaults(local, remote vault.VaultData) vault.VaultData {
	return MergeThreeWay(vault.VaultData{}, local, remote, local.Metadata.DeviceID, remote.Metadata.DeviceID)
}

func MergeThreeWay(base, local, remote vault.VaultData, localDeviceID, remoteDeviceID string) vault.VaultData {
	merged := vault.VaultData{
		Metadata:      mergeMetadata(base.Metadata, local.Metadata, remote.Metadata),
		KeyGeneration: maxInt(base.KeyGeneration, local.KeyGeneration, remote.KeyGeneration),
		SSH: vault.SSHData{
			Routes: mergeSSHRoutes(base.SSH.Routes, local.SSH.Routes, remote.SSH.Routes),
		},
	}

	merged.VersionVector = mergeVersionVectors(base.VersionVector, local.VersionVector, remote.VersionVector)
	merged.Tombstones = mergeTombstones(base.Tombstones, local.Tombstones, remote.Tombstones)

	tombstoneSet := make(map[string]struct{}, len(merged.Tombstones))
	for _, t := range merged.Tombstones {
		tombstoneSet[t.KeyID] = struct{}{}
	}

	baseKeys := keysByID(base.Keys)
	localKeys := keysByID(local.Keys)
	remoteKeys := keysByID(remote.Keys)

	ids := make(map[string]struct{}, len(baseKeys)+len(localKeys)+len(remoteKeys))
	for id := range baseKeys {
		ids[id] = struct{}{}
	}
	for id := range localKeys {
		ids[id] = struct{}{}
	}
	for id := range remoteKeys {
		ids[id] = struct{}{}
	}

	merged.Keys = make([]vault.Key, 0, len(ids))
	for id := range ids {
		if _, deleted := tombstoneSet[id]; deleted {
			continue
		}

		baseKey, hasBase := baseKeys[id]
		localKey, hasLocal := localKeys[id]
		remoteKey, hasRemote := remoteKeys[id]

		switch {
		case hasLocal && hasRemote:
			if hasBase {
				merged.Keys = append(merged.Keys, mergeKey(&baseKey, &localKey, &remoteKey, localDeviceID, remoteDeviceID))
			} else {
				merged.Keys = append(merged.Keys, mergeKey(nil, &localKey, &remoteKey, localDeviceID, remoteDeviceID))
			}
		case hasLocal:
			merged.Keys = append(merged.Keys, cloneKey(localKey))
		case hasRemote:
			merged.Keys = append(merged.Keys, cloneKey(remoteKey))
		}
	}

	merged.Keys = enforceGitSigningInvariant(merged.Keys, localDeviceID, remoteDeviceID)
	sortKeys(merged.Keys)
	return merged
}

func BootstrapMerge(local, remote vault.VaultData, localDeviceID, remoteDeviceID string) vault.VaultData {
	merged := vault.VaultData{
		Metadata:      mergeMetadata(vault.Metadata{}, local.Metadata, remote.Metadata),
		KeyGeneration: maxInt(local.KeyGeneration, remote.KeyGeneration),
		VersionVector: mergeVersionVectors(local.VersionVector, remote.VersionVector),
		Tombstones:    mergeTombstones(local.Tombstones, remote.Tombstones),
		SSH: vault.SSHData{
			Routes: mergeSSHRoutes(nil, local.SSH.Routes, remote.SSH.Routes),
		},
	}

	tombstoneSet := make(map[string]struct{}, len(merged.Tombstones))
	for _, t := range merged.Tombstones {
		tombstoneSet[t.KeyID] = struct{}{}
	}

	usedRemote := make([]bool, len(remote.Keys))
	for _, localKey := range local.Keys {
		matchIdx := findBootstrapMatch(localKey, remote.Keys, usedRemote)
		if matchIdx >= 0 {
			usedRemote[matchIdx] = true
			mergedKey := mergeKey(nil, &localKey, &remote.Keys[matchIdx], localDeviceID, remoteDeviceID)
			mergedKey.ID = remote.Keys[matchIdx].ID
			if _, deleted := tombstoneSet[mergedKey.ID]; !deleted {
				merged.Keys = append(merged.Keys, mergedKey)
			}
			continue
		}

		if _, deleted := tombstoneSet[localKey.ID]; deleted {
			continue
		}
		merged.Keys = append(merged.Keys, cloneKey(localKey))
	}

	for i, remoteKey := range remote.Keys {
		if usedRemote[i] {
			continue
		}
		if _, deleted := tombstoneSet[remoteKey.ID]; deleted {
			continue
		}
		merged.Keys = append(merged.Keys, cloneKey(remoteKey))
	}

	merged.Keys = enforceGitSigningInvariant(merged.Keys, localDeviceID, remoteDeviceID)
	sortKeys(merged.Keys)
	return merged
}

func mergeMetadata(base, local, remote vault.Metadata) vault.Metadata {
	merged := local
	if merged.DeviceID == "" {
		merged.DeviceID = firstNonEmpty(remote.DeviceID, base.DeviceID)
	}
	if merged.DeviceName == "" {
		merged.DeviceName = firstNonEmpty(remote.DeviceName, base.DeviceName)
	}
	merged.CreatedAt = earliestTime(base.CreatedAt, local.CreatedAt, remote.CreatedAt)
	return merged
}

func mergeVersionVectors(vectors ...map[string]int64) map[string]int64 {
	merged := make(map[string]int64)
	for _, vector := range vectors {
		for key, value := range vector {
			if value > merged[key] {
				merged[key] = value
			}
		}
	}
	return merged
}

func mergeTombstones(sets ...[]vault.Tombstone) []vault.Tombstone {
	seen := make(map[string]vault.Tombstone)
	cutoff := time.Now().UTC().Add(-tombstoneTTL)

	for _, tombstones := range sets {
		for _, tombstone := range tombstones {
			if tombstone.DeletedAt.After(cutoff) {
				if existing, ok := seen[tombstone.KeyID]; !ok || tombstone.DeletedAt.After(existing.DeletedAt) {
					seen[tombstone.KeyID] = tombstone
				}
			}
		}
	}

	result := make([]vault.Tombstone, 0, len(seen))
	for _, t := range seen {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DeletedAt.Equal(result[j].DeletedAt) {
			return result[i].KeyID < result[j].KeyID
		}
		return result[i].DeletedAt.Before(result[j].DeletedAt)
	})
	return result
}

func keysByID(keys []vault.Key) map[string]vault.Key {
	byID := make(map[string]vault.Key, len(keys))
	for _, key := range keys {
		if key.ID == "" {
			continue
		}
		byID[key.ID] = cloneKey(key)
	}
	return byID
}

func mergeKey(base, local, remote *vault.Key, localDeviceID, remoteDeviceID string) vault.Key {
	winner, loser := chooseWinner(local, remote, localDeviceID, remoteDeviceID)
	merged := cloneKey(*winner)
	merged.CreatedAt = earliestTime(keyTime(base, func(k *vault.Key) time.Time { return k.CreatedAt }), local.CreatedAt, remote.CreatedAt)
	merged.Version = maxInt(keyVersion(base), local.Version, remote.Version)
	merged.DeviceOrigin = firstNonEmpty(merged.DeviceOrigin, local.DeviceOrigin, remote.DeviceOrigin)
	merged.Tags = mergeStringSet(keyTags(base), local.Tags, remote.Tags)
	preservePrivateKey(&merged, base, local, remote, loser)
	return merged
}

func chooseWinner(local, remote *vault.Key, localDeviceID, remoteDeviceID string) (*vault.Key, *vault.Key) {
	if preferLocal(*local, *remote, localDeviceID, remoteDeviceID) {
		return local, remote
	}
	return remote, local
}

func preferLocal(local, remote vault.Key, localDeviceID, remoteDeviceID string) bool {
	if local.UpdatedAt.After(remote.UpdatedAt) {
		return true
	}
	if remote.UpdatedAt.After(local.UpdatedAt) {
		return false
	}
	if local.Version != remote.Version {
		return local.Version > remote.Version
	}
	return tieBreakID(local, localDeviceID) <= tieBreakID(remote, remoteDeviceID)
}

func mergeStringSet(base, local, remote []string) []string {
	baseSet := stringSet(base)
	localSet := stringSet(local)
	remoteSet := stringSet(remote)

	all := make(map[string]struct{}, len(baseSet)+len(localSet)+len(remoteSet))
	for value := range baseSet {
		all[value] = struct{}{}
	}
	for value := range localSet {
		all[value] = struct{}{}
	}
	for value := range remoteSet {
		all[value] = struct{}{}
	}

	result := make([]string, 0, len(all))
	for value := range all {
		_, inBase := baseSet[value]
		_, inLocal := localSet[value]
		_, inRemote := remoteSet[value]

		if inBase {
			if inLocal && inRemote {
				result = append(result, value)
			}
			continue
		}
		if inLocal || inRemote {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func preservePrivateKey(merged *vault.Key, candidates ...*vault.Key) {
	for _, candidate := range candidates {
		if candidate == nil || len(candidate.PrivateKey) == 0 {
			continue
		}
		if sameKeyMaterial(*merged, *candidate) {
			merged.PrivateKey = append([]byte(nil), candidate.PrivateKey...)
			return
		}
	}
	merged.PrivateKey = nil
}

func sameKeyMaterial(a, b vault.Key) bool {
	return a.PublicKey == b.PublicKey &&
		a.Fingerprint == b.Fingerprint &&
		a.EncryptedPrivateKey == b.EncryptedPrivateKey &&
		a.EncryptedCipherKey == b.EncryptedCipherKey
}

func cloneKey(key vault.Key) vault.Key {
	cloned := key
	cloned.PrivateKey = append([]byte(nil), key.PrivateKey...)
	cloned.Tags = append([]string(nil), key.Tags...)
	if key.LastUsedAt != nil {
		lastUsedAt := *key.LastUsedAt
		cloned.LastUsedAt = &lastUsedAt
	}
	return cloned
}

func enforceGitSigningInvariant(keys []vault.Key, localDeviceID, remoteDeviceID string) []vault.Key {
	winner := -1
	for i := range keys {
		if !keys[i].GitSigning {
			continue
		}
		if winner < 0 || preferLocal(keys[i], keys[winner], localDeviceID, remoteDeviceID) {
			winner = i
		}
	}
	if winner < 0 {
		return keys
	}
	for i := range keys {
		keys[i].GitSigning = i == winner
	}
	return keys
}

func sortKeys(keys []vault.Key) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Name == keys[j].Name {
			return keys[i].ID < keys[j].ID
		}
		return keys[i].Name < keys[j].Name
	})
}

func findBootstrapMatch(localKey vault.Key, remoteKeys []vault.Key, used []bool) int {
	for i, remoteKey := range remoteKeys {
		if used[i] {
			continue
		}
		if localKey.ID != "" && localKey.ID == remoteKey.ID {
			return i
		}
	}
	for i, remoteKey := range remoteKeys {
		if used[i] {
			continue
		}
		if localKey.Fingerprint != "" && localKey.Fingerprint == remoteKey.Fingerprint {
			return i
		}
	}
	for i, remoteKey := range remoteKeys {
		if used[i] {
			continue
		}
		if localKey.PublicKey != "" && localKey.PublicKey == remoteKey.PublicKey {
			return i
		}
	}
	return -1
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func keyTags(key *vault.Key) []string {
	if key == nil {
		return nil
	}
	return key.Tags
}

func keyVersion(key *vault.Key) int {
	if key == nil {
		return 0
	}
	return key.Version
}

func keyTime(key *vault.Key, field func(*vault.Key) time.Time) time.Time {
	if key == nil {
		return time.Time{}
	}
	return field(key)
}

func earliestTime(times ...time.Time) time.Time {
	var earliest time.Time
	for _, value := range times {
		if value.IsZero() {
			continue
		}
		if earliest.IsZero() || value.Before(earliest) {
			earliest = value
		}
	}
	return earliest
}

func maxInt(values ...int) int {
	current := 0
	for i, value := range values {
		if i == 0 || value > current {
			current = value
		}
	}
	return current
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func tieBreakID(key vault.Key, fallback string) string {
	value := firstNonEmpty(key.DeviceOrigin, key.ID, fallback)
	if value == "" {
		return strings.Repeat("~", 32)
	}
	return value
}

func mergeSSHRoutes(base, local, remote map[string]vault.SSHRoute) map[string]vault.SSHRoute {
	ids := make(map[string]struct{}, len(base)+len(local)+len(remote))
	for target := range base {
		ids[target] = struct{}{}
	}
	for target := range local {
		ids[target] = struct{}{}
	}
	for target := range remote {
		ids[target] = struct{}{}
	}

	out := make(map[string]vault.SSHRoute, len(ids))
	for target := range ids {
		route, ok := newestRoute(
			local[target],
			remote[target],
			base[target],
			local == nil || !routeExists(local, target),
			remote == nil || !routeExists(remote, target),
			base == nil || !routeExists(base, target),
		)
		if ok {
			out[target] = route
		}
	}
	return out
}

func newestRoute(local, remote, base vault.SSHRoute, localMissing, remoteMissing, baseMissing bool) (vault.SSHRoute, bool) {
	candidates := make([]vault.SSHRoute, 0, 3)
	if !baseMissing {
		candidates = append(candidates, base)
	}
	if !remoteMissing {
		candidates = append(candidates, remote)
	}
	if !localMissing {
		candidates = append(candidates, local)
	}
	if len(candidates) == 0 {
		return vault.SSHRoute{}, false
	}

	newest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Updated.After(newest.Updated) || candidate.Updated.Equal(newest.Updated) {
			newest = candidate
		}
	}
	return newest, true
}

func routeExists(routes map[string]vault.SSHRoute, target string) bool {
	if routes == nil {
		return false
	}
	_, ok := routes[target]
	return ok
}
