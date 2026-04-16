package vault

import (
	"fmt"
	"time"
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
	if target == "" {
		return fmt.Errorf("empty ssh route target")
	}
	if fingerprint == "" {
		return fmt.Errorf("empty ssh route key")
	}

	ks.mu.Lock()
	defer ks.mu.Unlock()

	originalRoutes := cloneSSHRoutes(ks.vault.Data.SSH.Routes)
	originalVersionVector := cloneVersionVector(ks.vault.Data.VersionVector)

	ensureSSHRoutes(&ks.vault.Data)
	ks.vault.Data.SSH.Routes[target] = SSHRoute{
		Key:     fingerprint,
		Updated: updated.UTC(),
	}
	ks.bumpVersionVector()

	if err := ks.vault.Save(); err != nil {
		ks.vault.Data.SSH.Routes = originalRoutes
		ks.vault.Data.VersionVector = originalVersionVector
		return fmt.Errorf("saving vault: %w", err)
	}

	return nil
}

func (ks *KeyStore) ClearSSHRoute(target string, updated time.Time) error {
	if target == "" {
		return fmt.Errorf("empty ssh route target")
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
		return fmt.Errorf("saving vault: %w", err)
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
		cloned[target] = route
	}
	return cloned
}
