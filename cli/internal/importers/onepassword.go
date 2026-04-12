package importers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type onePasswordExport struct {
	Accounts []struct {
		Vaults []struct {
			Items []onePasswordItem `json:"items"`
		} `json:"vaults"`
	} `json:"accounts"`
}

type onePasswordItem struct {
	CategoryUUID string `json:"categoryUuid"`
	Overview     struct {
		Title string `json:"title"`
	} `json:"overview"`
	Details struct {
		Sections []struct {
			Fields []struct {
				Title string          `json:"title"`
				Value json.RawMessage `json:"value"`
			} `json:"fields"`
		} `json:"sections"`
	} `json:"details"`
}

func Parse1Password(data []byte) ([]ImportedKey, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening 1pux archive: %w", err)
	}

	var exportData []byte
	for _, f := range reader.File {
		if f.Name == "export.data" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("reading export.data: %w", err)
			}
			exportData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("reading export.data: %w", err)
			}
			break
		}
	}
	if exportData == nil {
		return nil, fmt.Errorf("export.data not found in archive")
	}

	var export onePasswordExport
	if err := json.Unmarshal(exportData, &export); err != nil {
		return nil, fmt.Errorf("parsing export.data: %w", err)
	}

	var keys []ImportedKey
	for _, account := range export.Accounts {
		for _, vault := range account.Vaults {
			for _, item := range vault.Items {
				if item.CategoryUUID != "114" {
					continue
				}
				privKey := extractOnePasswordSSHKey(item)
				if privKey == "" {
					continue
				}
				keys = append(keys, ImportedKey{
					Name:       SanitizeName(item.Overview.Title),
					PrivateKey: privKey,
				})
			}
		}
	}
	return keys, nil
}

func extractOnePasswordSSHKey(item onePasswordItem) string {
	for _, section := range item.Details.Sections {
		for _, field := range section.Fields {
			var val map[string]json.RawMessage
			if err := json.Unmarshal(field.Value, &val); err != nil {
				continue
			}
			if sshKeyRaw, ok := val["sshKey"]; ok {
				var sshKey struct {
					PrivateKey string `json:"privateKey"`
				}
				if err := json.Unmarshal(sshKeyRaw, &sshKey); err == nil && sshKey.PrivateKey != "" {
					return sshKey.PrivateKey
				}
			}
			if strings.Contains(strings.ToLower(field.Title), "private key") {
				var plain string
				if err := json.Unmarshal(field.Value, &plain); err == nil && strings.Contains(plain, "PRIVATE KEY") {
					return plain
				}
			}
		}
	}
	return ""
}
