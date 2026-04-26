package sshrouting

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itzzritik/forged/cli/internal/vault"
)

const (
	keyRefDomain = "forged-key-ref-v1:"
	minKeyRefLen = 4
)

type KeyRef struct {
	Fingerprint string
	PublicKey   string
	Name        string
	Ref         string
	Path        string
}

func BuildKeyRefs(keys []vault.Key, dir string) ([]KeyRef, error) {
	byFingerprint := make(map[string]vault.Key, len(keys))
	for _, key := range keys {
		fingerprint := strings.TrimSpace(key.Fingerprint)
		if fingerprint == "" || strings.TrimSpace(key.PublicKey) == "" {
			continue
		}
		if _, exists := byFingerprint[fingerprint]; !exists {
			byFingerprint[fingerprint] = key
		}
	}

	fingerprints := make([]string, 0, len(byFingerprint))
	for fingerprint := range byFingerprint {
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Strings(fingerprints)

	digests := make(map[string]string, len(fingerprints))
	for _, fingerprint := range fingerprints {
		digests[fingerprint] = keyRefDigest(fingerprint)
	}

	lengths := make(map[string]int, len(fingerprints))
	for _, fingerprint := range fingerprints {
		lengths[fingerprint] = minKeyRefLen
	}
	for {
		buckets := make(map[string][]string, len(fingerprints))
		collided := false
		for _, fingerprint := range fingerprints {
			digest := digests[fingerprint]
			length := lengths[fingerprint]
			if length > len(digest) {
				return nil, fmt.Errorf("Could not derive unique ref for fingerprint %q", fingerprint)
			}
			buckets[digest[:length]] = append(buckets[digest[:length]], fingerprint)
		}
		for _, bucket := range buckets {
			if len(bucket) < 2 {
				continue
			}
			collided = true
			for _, fingerprint := range bucket {
				lengths[fingerprint]++
			}
		}
		if !collided {
			break
		}
		for _, fingerprint := range fingerprints {
			if lengths[fingerprint] > len(digests[fingerprint]) {
				collided = true
				break
			}
		}
	}

	refs := make([]KeyRef, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		key := byFingerprint[fingerprint]
		ref := "k_" + digests[fingerprint][:lengths[fingerprint]]
		refs = append(refs, KeyRef{
			Fingerprint: fingerprint,
			PublicKey:   strings.TrimSpace(key.PublicKey),
			Name:        key.Name,
			Ref:         ref,
			Path:        filepath.Join(dir, ref+".pub"),
		})
	}
	return refs, nil
}

func KeyRefsByFingerprint(refs []KeyRef) map[string]KeyRef {
	out := make(map[string]KeyRef, len(refs))
	for _, ref := range refs {
		out[ref.Fingerprint] = ref
	}
	return out
}

func keyRefDigest(fingerprint string) string {
	sum := sha256.Sum256([]byte(keyRefDomain + fingerprint))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return strings.ToLower(encoded)
}
