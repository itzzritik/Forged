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
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	route, ok := ks.vault.Data.SSH.Routes[target]
	return route, ok
}

func (ks *KeyStore) SSHRoutes() map[string]SSHRoute {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	return cloneSSHRoutes(ks.vault.Data.SSH.Routes)
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

	ks.mu.Lock()
	defer ks.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.Data.SSH.Routes)
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)

	ensureSSHRoutes(&ks.vault.Data)
	previous := ks.vault.Data.SSH.Routes[target]
	successCount := previous.SuccessCount
	if previous.Key == fingerprint {
		successCount++
	} else {
		successCount = 1
	}
	successAt := updated.UTC()
	ks.vault.Data.SSH.Routes[target] = SSHRoute{
		Key:           fingerprint,
		Updated:       updated.UTC(),
		ProvenBy:      provenBy,
		Operation:     operation,
		SuccessCount:  successCount,
		LastSuccessAt: &successAt,
		Attempts:      cloneRouteAttempts(previous.Attempts),
	}
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.SSH.Routes = originalRoutes
		ks.vault.Data.VersionVector = originalVersionVector
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

	ks.mu.Lock()
	defer ks.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.Data.SSH.Routes)
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)

	ensureSSHRoutes(&ks.vault.Data)
	route := cloneSSHRoute(ks.vault.Data.SSH.Routes[target])
	if route.Attempts == nil {
		route.Attempts = map[string]time.Time{}
	}
	route.Attempts[fingerprint] = updated.UTC()
	route.Operation = operation
	route.Updated = updated.UTC()
	ks.vault.Data.SSH.Routes[target] = route
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.SSH.Routes = originalRoutes
		ks.vault.Data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) ClearSSHRoute(target string, updated time.Time) error {
	if target == "" {
		return fmt.Errorf("Empty SSH route target")
	}

	ks.mu.Lock()
	defer ks.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.Data.SSH.Routes)
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)

	ensureSSHRoutes(&ks.vault.Data)
	ks.vault.Data.SSH.Routes[target] = SSHRoute{
		Updated: updated.UTC(),
	}
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.SSH.Routes = originalRoutes
		ks.vault.Data.VersionVector = originalVersionVector
		return fmt.Errorf("Saving vault: %w", err)
	}

	return nil
}

func ensureSSHRoutes(data *VaultData) {
	if data.SSH.Routes == nil {
		data.SSH.Routes = map[string]SSHRoute{}
	}
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
