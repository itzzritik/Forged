package sshrouting

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"

	"github.com/itzzritik/forged/cli/internal/vault"
)

type RouteKeyRef struct {
	Ref       string
	PublicKey string
}

func BuildRouteKeyRefs(keys []vault.Key) (map[string]string, []RouteKeyRef) {
	type fingerprintKey struct {
		fingerprint string
		publicKey   string
		token       string
	}

	byFingerprint := make(map[string]fingerprintKey)
	for _, key := range keys {
		if _, ok := byFingerprint[key.Fingerprint]; ok {
			continue
		}
		byFingerprint[key.Fingerprint] = fingerprintKey{
			fingerprint: key.Fingerprint,
			publicKey:   strings.TrimSpace(key.PublicKey),
			token:       normalizedFingerprintToken(key.Fingerprint),
		}
	}

	type item struct {
		fingerprint string
		publicKey   string
		token       string
		ref         string
	}

	items := make([]item, 0, len(byFingerprint))
	tokenCounts := make(map[string]int, len(byFingerprint))
	for _, candidate := range byFingerprint {
		tokenCounts[candidate.token]++
		items = append(items, item{
			fingerprint: candidate.fingerprint,
			publicKey:   candidate.publicKey,
			token:       candidate.token,
		})
	}

	for i := range items {
		if tokenCounts[items[i].token] == 1 {
			continue
		}
		items[i].token = items[i].token + stableKeyIDSuffix(items[i].fingerprint)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].token == items[j].token {
			return items[i].fingerprint < items[j].fingerprint
		}
		return items[i].token < items[j].token
	})

	tokens := make([]string, 0, len(items))
	for _, item := range items {
		tokens = append(tokens, item.token)
	}
	for i := range items {
		items[i].ref = "gh-" + shortestUniquePrefix(items[i].token, tokens, 6)
	}

	fingerprintToRef := make(map[string]string, len(items))
	routed := make([]RouteKeyRef, 0, len(items))
	for _, item := range items {
		fingerprintToRef[item.fingerprint] = item.ref
		routed = append(routed, RouteKeyRef{
			Ref:       item.ref,
			PublicKey: item.publicKey,
		})
	}

	idToRef := make(map[string]string, len(keys))
	for _, key := range keys {
		idToRef[key.ID] = fingerprintToRef[key.Fingerprint]
	}

	return idToRef, routed
}

func normalizedFingerprintToken(fingerprint string) string {
	base := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fingerprint), "SHA256:"))
	var b strings.Builder
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	token := b.String()
	if token == "" {
		return "key"
	}
	return token
}

func stableKeyIDSuffix(fingerprint string) string {
	sum := sha1.Sum([]byte(fingerprint))
	return "-" + hex.EncodeToString(sum[:])[:6]
}

func shortestUniquePrefix(token string, all []string, min int) string {
	if len(token) <= min {
		return token
	}
	for length := min; length <= len(token); length++ {
		prefix := token[:length]
		unique := true
		for _, other := range all {
			if other == token {
				continue
			}
			if strings.HasPrefix(other, prefix) {
				unique = false
				break
			}
		}
		if unique {
			return prefix
		}
	}
	return token
}
