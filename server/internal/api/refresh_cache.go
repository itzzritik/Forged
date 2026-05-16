package api

import (
	"encoding/hex"
	"sync"
	"time"

	serverauth "github.com/itzzritik/forged/server/internal/auth"
)

// rotationCache memoizes the response of a successful refresh-rotation for a
// short grace window. If the same refresh secret is presented twice within
// RefreshGracePeriod, the second call serves the cached response instead of
// triggering family-revoke. This absorbs honest client retries (network
// hiccups, near-simultaneous requests) while preserving replay detection
// outside the window.
type rotationCache struct {
	mu      sync.Mutex
	entries map[string]rotationCacheEntry
}

type rotationCacheEntry struct {
	response  map[string]any
	expiresAt time.Time
}

func newRotationCache() *rotationCache {
	return &rotationCache{entries: make(map[string]rotationCacheEntry)}
}

// key derives the cache key from the secret hash that the client presented.
// Hashing was already done before lookup, so we just hex-encode for use as a
// map key.
func rotationKey(presentedSecretHash []byte) string {
	return hex.EncodeToString(presentedSecretHash)
}

func (c *rotationCache) lookup(presentedSecretHash []byte) (map[string]any, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()
	entry, ok := c.entries[rotationKey(presentedSecretHash)]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, rotationKey(presentedSecretHash))
		return nil, false
	}
	return entry.response, true
}

func (c *rotationCache) store(presentedSecretHash []byte, response map[string]any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()
	c.entries[rotationKey(presentedSecretHash)] = rotationCacheEntry{
		response:  response,
		expiresAt: time.Now().Add(serverauth.RefreshGracePeriod),
	}
}

// pruneLocked removes entries whose grace window has expired. Caller must
// hold the mutex. Cheap because the cache only holds in-flight grace entries
// (bounded by grace period × refresh rate).
func (c *rotationCache) pruneLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}
