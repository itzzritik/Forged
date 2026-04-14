package sshrouting

import "github.com/itzzritik/forged/cli/internal/vault"

type LegacyHint struct {
	Host  string
	KeyID string
}

func LegacyHints(keys []vault.Key) []LegacyHint {
	var out []LegacyHint
	for _, key := range keys {
		for _, rule := range key.HostRules {
			if rule.Match == "" {
				continue
			}
			out = append(out, LegacyHint{
				Host:  rule.Match,
				KeyID: key.ID,
			})
		}
	}
	return out
}
