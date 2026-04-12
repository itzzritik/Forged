package importers

import (
	"encoding/json"
	"fmt"
)

type bitwardenExport struct {
	Encrypted bool            `json:"encrypted"`
	Items     []bitwardenItem `json:"items"`
}

type bitwardenItem struct {
	Type   int    `json:"type"`
	Name   string `json:"name"`
	SSHKey *struct {
		PrivateKey     string `json:"privateKey"`
		PublicKey      string `json:"publicKey"`
		KeyFingerprint string `json:"keyFingerprint"`
	} `json:"sshKey"`
}

func ParseBitwarden(data []byte) ([]ImportedKey, error) {
	var export bitwardenExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parsing Bitwarden export: %w", err)
	}
	if export.Encrypted {
		return nil, fmt.Errorf("encrypted Bitwarden exports are not supported -- export as unencrypted JSON")
	}

	var keys []ImportedKey
	for _, item := range export.Items {
		if item.Type != 5 || item.SSHKey == nil || item.SSHKey.PrivateKey == "" {
			continue
		}
		keys = append(keys, ImportedKey{
			Name:       SanitizeName(item.Name),
			PrivateKey: item.SSHKey.PrivateKey,
			PublicKey:  item.SSHKey.PublicKey,
		})
	}
	return keys, nil
}
