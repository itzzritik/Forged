package vault

import (
	"fmt"
	"time"
)

const (
	SSHRouteProofProviderProbe = "provider_probe"
	SSHRouteProofSSHAuth       = "ssh_auth"
)

func (ks *KeyStore) SSHRoute(target string) (SSHRoute, bool) {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	route, ok := ks.vault.data.SSH.Routes[target]
	if !ok || routeDeletedByTombstone(route, ks.vault.data.SSH.Tombstones, target) {
		return SSHRoute{}, false
	}
	return route, ok
}

func (ks *KeyStore) SSHRoutes() map[string]SSHRoute {
	ks.vault.mu.RLock()
	defer ks.vault.mu.RUnlock()

	return effectiveSSHRoutes(ks.vault.data.SSH.Routes, ks.vault.data.SSH.Tombstones)
}

func (ks *KeyStore) RecordSSHRoute(target, fingerprint string, updated time.Time) error {
	return ks.RecordSSHRouteProof(target, fingerprint, SSHRouteProofSSHAuth, "", updated)
}

func (ks *KeyStore) RecordSSHRouteProof(target, fingerprint, provenBy, operation string, updated time.Time) error {
	if target == "" {
		return fmt.Errorf("Empty SSH route target")
	}
	if fingerprint == "" {
		return fmt.Errorf("Empty SSH route key")
	}

	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.data.SSH.Routes)
	originalTombstones := cloneSSHRouteTombstones(ks.vault.data.SSH.Tombstones)
	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)

	ensureSSHData(&ks.vault.data)
	if routeDeletedAtOrAfter(ks.vault.data.SSH.Tombstones, target, updated.UTC()) {
		return nil
	}
	previous := ks.vault.data.SSH.Routes[target]
	successCount := previous.SuccessCount
	if previous.Key == fingerprint {
		successCount++
	} else {
		successCount = 1
	}
	successAt := updated.UTC()
	ks.vault.data.SSH.Routes[target] = SSHRoute{
		Key:           fingerprint,
		Updated:       updated.UTC(),
		ProvenBy:      provenBy,
		Operation:     operation,
		SuccessCount:  successCount,
		LastSuccessAt: &successAt,
		Attempts:      cloneRouteAttempts(previous.Attempts),
	}
	ks.removeSSHRouteTombstone(target, updated.UTC())
	ks.bumpVersionVector()

	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.SSH.Routes = originalRoutes
		ks.vault.data.SSH.Tombstones = originalTombstones
		ks.vault.data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) RecordSSHRouteAttempt(target, fingerprint, operation string, updated time.Time) error {
	if target == "" {
		return fmt.Errorf("Empty SSH route target")
	}
	if fingerprint == "" {
		return fmt.Errorf("Empty SSH route key")
	}

	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.data.SSH.Routes)
	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)

	ensureSSHData(&ks.vault.data)
	route := cloneSSHRoute(ks.vault.data.SSH.Routes[target])
	if route.Attempts == nil {
		route.Attempts = map[string]time.Time{}
	}
	route.Attempts[fingerprint] = updated.UTC()
	route.Operation = operation
	route.Updated = updated.UTC()
	ks.vault.data.SSH.Routes[target] = route
	ks.bumpVersionVector()

	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.SSH.Routes = originalRoutes
		ks.vault.data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) ClearSSHRoute(target string, updated time.Time) error {
	if target == "" {
		return fmt.Errorf("Empty SSH route target")
	}

	ks.vault.mu.Lock()
	defer ks.vault.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.data.SSH.Routes)
	originalTombstones := cloneSSHRouteTombstones(ks.vault.data.SSH.Tombstones)
	originalVersionVector := cloneVersionVector(ks.vault.data.VersionVector)

	ensureSSHData(&ks.vault.data)
	delete(ks.vault.data.SSH.Routes, target)
	ks.upsertSSHRouteTombstone(target, updated.UTC())
	ks.bumpVersionVector()

	if err := ks.vault.saveLocked(); err != nil {
		ks.vault.data.SSH.Routes = originalRoutes
		ks.vault.data.SSH.Tombstones = originalTombstones
		ks.vault.data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func ensureSSHRoutes(data *VaultData) {
	ensureSSHData(data)
}

func ensureSSHData(data *VaultData) {
	if data.SSH.Routes == nil {
		data.SSH.Routes = map[string]SSHRoute{}
	}
}

func effectiveSSHRoutes(routes map[string]SSHRoute, tombstones []SSHRouteTombstone) map[string]SSHRoute {
	cloned := make(map[string]SSHRoute, len(routes))
	for target, route := range routes {
		if routeDeletedByTombstone(route, tombstones, target) {
			continue
		}
		cloned[target] = cloneSSHRoute(route)
	}
	return cloned
}

func cloneSSHRoutes(routes map[string]SSHRoute) map[string]SSHRoute {
	cloned := make(map[string]SSHRoute, len(routes))
	for target, route := range routes {
		cloned[target] = cloneSSHRoute(route)
	}
	return cloned
}

func cloneSSHRoute(route SSHRoute) SSHRoute {
	cloned := route
	if route.LastSuccessAt != nil {
		lastSuccessAt := *route.LastSuccessAt
		cloned.LastSuccessAt = &lastSuccessAt
	}
	cloned.Attempts = cloneRouteAttempts(route.Attempts)
	return cloned
}

func cloneSSHRouteTombstones(tombstones []SSHRouteTombstone) []SSHRouteTombstone {
	cloned := make([]SSHRouteTombstone, len(tombstones))
	copy(cloned, tombstones)
	return cloned
}

func routeDeletedByTombstone(route SSHRoute, tombstones []SSHRouteTombstone, target string) bool {
	if route.Key == "" && len(route.Attempts) == 0 {
		return true
	}
	for _, tombstone := range tombstones {
		if tombstone.Target != target {
			continue
		}
		if route.Key == "" {
			return true
		}
		if tombstone.DeletedAt.After(route.Updated) || tombstone.DeletedAt.Equal(route.Updated) {
			return true
		}
	}
	return false
}

func routeDeletedAtOrAfter(tombstones []SSHRouteTombstone, target string, at time.Time) bool {
	for _, tombstone := range tombstones {
		if tombstone.Target == target && (tombstone.DeletedAt.After(at) || tombstone.DeletedAt.Equal(at)) {
			return true
		}
	}
	return false
}

func (ks *KeyStore) upsertSSHRouteTombstone(target string, deletedAt time.Time) {
	tombstone := SSHRouteTombstone{
		Target:          target,
		DeletedAt:       deletedAt.UTC(),
		DeletedByDevice: ks.vault.data.Metadata.DeviceID,
	}

	for i := range ks.vault.data.SSH.Tombstones {
		if ks.vault.data.SSH.Tombstones[i].Target != target {
			continue
		}
		if deletedAt.After(ks.vault.data.SSH.Tombstones[i].DeletedAt) {
			ks.vault.data.SSH.Tombstones[i] = tombstone
		}
		return
	}

	ks.vault.data.SSH.Tombstones = append(ks.vault.data.SSH.Tombstones, tombstone)
}

func (ks *KeyStore) removeSSHRouteTombstone(target string, routeUpdated time.Time) {
	tombstones := ks.vault.data.SSH.Tombstones
	kept := tombstones[:0]
	for _, tombstone := range tombstones {
		if tombstone.Target == target && routeUpdated.After(tombstone.DeletedAt) {
			continue
		}
		kept = append(kept, tombstone)
	}
	ks.vault.data.SSH.Tombstones = kept
}

func cloneRouteAttempts(attempts map[string]time.Time) map[string]time.Time {
	if len(attempts) == 0 {
		return nil
	}
	cloned := make(map[string]time.Time, len(attempts))
	for key, value := range attempts {
		cloned[key] = value
	}
	return cloned
}
