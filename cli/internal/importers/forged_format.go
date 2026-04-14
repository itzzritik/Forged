package importers

import (
	"encoding/json"
	"fmt"
)

type ForgedExport struct {
	Format     string        `json:"format"`
	Version    int           `json:"version"`
	ExportedAt string        `json:"exported_at"`
	Items      []ForgedItem  `json:"items"`
}

type ForgedItem struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	SSHKey *ForgedSSHKey  `json:"ssh_key,omitempty"`
	CreatedAt string      `json:"created_at,omitempty"`
	UpdatedAt string      `json:"updated_at,omitempty"`
}

type ForgedSSHKey struct {
	PrivateKey  string   `json:"private_key"`
	PublicKey   string   `json:"public_key"`
	Fingerprint string   `json:"fingerprint"`
	KeyType     string   `json:"key_type"`
	Comment     string   `json:"comment"`
	GitSigning bool `json:"git_signing"`
}

func ParseForged(data []byte) ([]ImportedKey, error) {
	var export ForgedExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parsing Forged export: %w", err)
	}
	if export.Format != "forged-export" {
		return nil, fmt.Errorf("not a Forged export file")
	}

	var keys []ImportedKey
	for _, item := range export.Items {
		if item.Type != "ssh_key" || item.SSHKey == nil || item.SSHKey.PrivateKey == "" {
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
