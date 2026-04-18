package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/itzzritik/forged/cli/internal/importers"
	"github.com/itzzritik/forged/cli/internal/ipc"
)

type ImportResult struct {
	Source     string
	Discovered int
	Imported   int
	Skipped    int
	Keys       []KeySummary
}

func ImportSourceLabel(source string) string {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "1password":
		return "1Password"
	case "bitwarden":
		return "Bitwarden"
	case "forged":
		return "Forged export"
	case "ssh-dir":
		return "SSH directory"
	case "file":
		return "Key file"
	default:
		return "Import"
	}
}

func ImportFromSource(paths config.Paths, source string, file string) (ImportResult, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	keys, err := loadImportedKeys(source, file)
	if err != nil {
		return ImportResult{}, err
	}

	result := ImportResult{
		Source:     source,
		Discovered: len(keys),
	}
	if len(keys) == 0 {
		return result, nil
	}

	client := ipc.NewClient(paths.CtlSocket())
	for _, key := range keys {
		resp, err := client.Call(ipc.CmdAdd, map[string]string{
			"name":        key.Name,
			"private_key": key.PrivateKey,
			"comment":     "",
		})
		if err != nil {
			result.Skipped++
			continue
		}
		result.Imported++

		var added struct {
			Name         string `json:"name"`
			ResolvedName string `json:"resolved_name"`
			Type         string `json:"type"`
			Fingerprint  string `json:"fingerprint"`
		}
		if err := json.Unmarshal(resp.Data, &added); err == nil {
			name := strings.TrimSpace(added.ResolvedName)
			if name == "" {
				name = strings.TrimSpace(added.Name)
			}
			if name != "" {
				result.Keys = append(result.Keys, KeySummary{
					Name:        name,
					Type:        strings.TrimSpace(added.Type),
					Fingerprint: strings.TrimSpace(added.Fingerprint),
				})
			}
		}
	}

	return result, nil
}

func loadImportedKeys(source string, file string) ([]importers.ImportedKey, error) {
	if source == "" {
		return nil, fmt.Errorf("choose an import source")
	}
	if source == "ssh-dir" {
		return importFromSSHDir(), nil
	}
	if strings.TrimSpace(file) == "" {
		return nil, fmt.Errorf("enter a file path")
	}

	data, err := os.ReadFile(expandUserPath(file))
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	switch source {
	case "1password":
		keys, err := importers.Parse1Password(data)
		if err != nil {
			return nil, fmt.Errorf("parsing 1Password export: %w", err)
		}
		return keys, nil
	case "bitwarden":
		keys, err := importers.ParseBitwarden(data)
		if err != nil {
			return nil, fmt.Errorf("parsing Bitwarden export: %w", err)
		}
		return keys, nil
	case "forged":
		keys, err := importers.ParseForged(data)
		if err != nil {
			return nil, fmt.Errorf("parsing Forged export: %w", err)
		}
		return keys, nil
	case "file":
		return []importers.ImportedKey{{
			Name:       deriveImportedKeyName(file),
			PrivateKey: string(data),
		}}, nil
	default:
		return nil, fmt.Errorf("unknown import source %q", source)
	}
}

func importFromSSHDir() []importers.ImportedKey {
	paths := hostmatch.DiscoverSSHKeys()
	keys := make([]importers.ImportedKey, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		keys = append(keys, importers.ImportedKey{
			Name:       deriveImportedKeyName(path),
			PrivateKey: string(data),
		})
	}
	return keys
}

func deriveImportedKeyName(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimPrefix(name, "id_")
	name = strings.TrimPrefix(name, "id-")
	name = strings.ReplaceAll(name, "_", " ")
	name = importers.SanitizeName(name)
	if name == "" {
		name = "Default"
	}
	return name
}

func expandUserPath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func loadExistingVaultFingerprints(paths config.Paths) (map[string]struct{}, error) {
	resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdList, nil)
	if err != nil {
		return nil, fmt.Errorf("loading existing keys: %w", err)
	}

	var result struct {
		Keys []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing existing keys: %w", err)
	}

	seen := make(map[string]struct{}, len(result.Keys))
	for _, key := range result.Keys {
		if strings.TrimSpace(key.Fingerprint) != "" {
			seen[key.Fingerprint] = struct{}{}
		}
	}
	return seen, nil
}
