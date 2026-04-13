package importers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var onePasswordPrivateKeyRE = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)

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
	trimmed := bytes.TrimSpace(data)
	if isZipArchive(trimmed) {
		return parse1Password1PUX(trimmed)
	}
	return parse1PasswordCSV(trimmed)
}

func parse1Password1PUX(data []byte) ([]ImportedKey, error) {
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
					return extractPrivateKeyBlock(sshKey.PrivateKey)
				}
			}
			if strings.Contains(strings.ToLower(field.Title), "private key") {
				var plain string
				if err := json.Unmarshal(field.Value, &plain); err == nil {
					if privateKey := extractPrivateKeyBlock(plain); privateKey != "" {
						return privateKey
					}
				}
			}
		}
	}
	return ""
}

func parse1PasswordCSV(data []byte) ([]ImportedKey, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing 1Password CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	titleIndex, start := detectOnePasswordCSVHeader(rows[0])
	keys := make([]ImportedKey, 0)
	for i, row := range rows[start:] {
		privateKey := extractPrivateKeyFromCSVRow(row)
		if privateKey == "" {
			continue
		}
		keys = append(keys, ImportedKey{
			Name:       deriveOnePasswordCSVName(row, titleIndex, i+1),
			PrivateKey: privateKey,
		})
	}
	return keys, nil
}

func isZipArchive(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' &&
		data[1] == 'K' &&
		((data[2] == 0x03 && data[3] == 0x04) ||
			(data[2] == 0x05 && data[3] == 0x06) ||
			(data[2] == 0x07 && data[3] == 0x08))
}

func detectOnePasswordCSVHeader(row []string) (int, int) {
	titleIndex := -1
	hasHeader := false

	for i, value := range row {
		switch normalizeOnePasswordCSVHeader(value) {
		case "title", "name":
			hasHeader = true
			titleIndex = i
		case "website", "url", "username", "password", "notes", "tags", "favorite", "archived", "one-timepassword":
			hasHeader = true
		}
	}

	if hasHeader {
		return titleIndex, 1
	}
	return -1, 0
}

func normalizeOnePasswordCSVHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func extractPrivateKeyFromCSVRow(row []string) string {
	for _, value := range row {
		if privateKey := extractPrivateKeyBlock(value); privateKey != "" {
			return privateKey
		}
	}
	return ""
}

func extractPrivateKeyBlock(value string) string {
	match := onePasswordPrivateKeyRE.FindString(value)
	if match == "" {
		return ""
	}
	return strings.TrimSpace(match)
}

func deriveOnePasswordCSVName(row []string, titleIndex int, ordinal int) string {
	if titleIndex >= 0 && titleIndex < len(row) {
		name := SanitizeName(row[titleIndex])
		if name != DefaultImportedName {
			return name
		}
	}

	for _, value := range row {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "PRIVATE KEY") {
			continue
		}
		name := SanitizeName(value)
		if name != DefaultImportedName {
			return name
		}
	}

	return FallbackImportedName(ordinal)
}
