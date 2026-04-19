package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/itzzritik/forged/cli/internal/importers"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/keytypes"
	"github.com/itzzritik/forged/cli/internal/vault"
)

type ImportResult struct {
	Source     string
	Discovered int
	Imported   int
	Skipped    int
	Keys       []KeySummary
}

type ImportPreview struct {
	Key         importers.ImportedKey
	Converted   bool
	Fingerprint string
	Selected    bool
}

type ImportPreviewResult struct {
	Source     string
	Discovered int
	Duplicates int
	Previews   []ImportPreview
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

	return importKeys(paths, ImportResult{
		Source:     source,
		Discovered: len(keys),
	}, keys)
}

func PreviewImportSource(paths config.Paths, source string, file string) (ImportPreviewResult, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	keys, err := loadImportedKeys(source, file)
	if err != nil {
		return ImportPreviewResult{}, err
	}

	result := ImportPreviewResult{
		Source:     source,
		Discovered: len(keys),
	}
	if len(keys) == 0 {
		return result, nil
	}

	existingFingerprints, err := loadExistingVaultFingerprints(paths)
	if err != nil {
		return ImportPreviewResult{}, err
	}

	previews, duplicates, err := buildImportPreview(keys, existingFingerprints)
	if err != nil {
		return ImportPreviewResult{}, err
	}
	result.Duplicates = duplicates
	result.Previews = previews
	return result, nil
}

func ImportSelectedPreviews(paths config.Paths, source string, discovered int, previews []ImportPreview) (ImportResult, error) {
	keys := make([]importers.ImportedKey, 0, len(previews))
	for _, preview := range previews {
		if preview.Selected {
			keys = append(keys, preview.Key)
		}
	}
	return importKeys(paths, ImportResult{
		Source:     strings.TrimSpace(strings.ToLower(source)),
		Discovered: discovered,
	}, keys)
}

func importKeys(paths config.Paths, result ImportResult, keys []importers.ImportedKey) (ImportResult, error) {
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
					Type:        keytypes.Normalize(added.Type),
					Fingerprint: strings.TrimSpace(added.Fingerprint),
				})
			}
		}
	}

	return result, nil
}

func buildImportPreview(keys []importers.ImportedKey, existingFingerprints map[string]struct{}) ([]ImportPreview, int, error) {
	previews := make([]ImportPreview, 0, len(keys))
	seenFingerprints := make(map[string]struct{}, len(keys))
	duplicates := 0
	for _, key := range keys {
		preview, err := previewImportedKey(key)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", key.Name, err)
		}
		if containsFingerprint(existingFingerprints, preview.Fingerprint) {
			duplicates++
			continue
		}
		if _, ok := seenFingerprints[preview.Fingerprint]; ok {
			duplicates++
			continue
		}
		preview.Selected = true
		seenFingerprints[preview.Fingerprint] = struct{}{}
		previews = append(previews, preview)
	}
	return previews, duplicates, nil
}

func previewImportedKey(key importers.ImportedKey) (ImportPreview, error) {
	normalized, err := vault.NormalizePrivateKeyToOpenSSH([]byte(key.PrivateKey), "")
	if err != nil {
		return ImportPreview{}, formatPrivateKeyImportError(err)
	}

	return ImportPreview{
		Key:         key,
		Converted:   normalized.Converted,
		Fingerprint: normalized.Fingerprint,
	}, nil
}

func formatPrivateKeyImportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, vault.ErrPassphraseProtectedPrivateKey) {
		return fmt.Errorf("Key is passphrase-protected. Remove the passphrase first with: ssh-keygen -p -f <file>")
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "private key is empty"):
		return fmt.Errorf("Private key is empty")
	case strings.Contains(msg, "parsing private key"):
		return fmt.Errorf("Unrecognized key format. Only unencrypted OpenSSH or PEM private keys are supported")
	case strings.Contains(msg, "creating signer from private key"):
		return fmt.Errorf("Unsupported private key type. Only RSA, ECDSA, and Ed25519 keys are supported")
	default:
		return err
	}
}

func loadImportedKeys(source string, file string) ([]importers.ImportedKey, error) {
	if source == "" {
		return nil, fmt.Errorf("Choose an import source")
	}
	if source == "ssh-dir" {
		return importFromSSHDir(), nil
	}
	if strings.TrimSpace(file) == "" {
		return nil, fmt.Errorf("Enter a file path")
	}

	data, err := os.ReadFile(expandUserPath(file))
	if err != nil {
		return nil, fmt.Errorf("Reading file: %w", err)
	}

	switch source {
	case "1password":
		keys, err := importers.Parse1Password(data)
		if err != nil {
			return nil, fmt.Errorf("Parsing 1Password export: %w", err)
		}
		return keys, nil
	case "bitwarden":
		keys, err := importers.ParseBitwarden(data)
		if err != nil {
			return nil, fmt.Errorf("Parsing Bitwarden export: %w", err)
		}
		return keys, nil
	case "forged":
		keys, err := importers.ParseForged(data)
		if err != nil {
			return nil, fmt.Errorf("Parsing Forged export: %w", err)
		}
		return keys, nil
	case "file":
		return []importers.ImportedKey{{
			Name:       deriveImportedKeyName(file),
			PrivateKey: string(data),
		}}, nil
	default:
		return nil, fmt.Errorf("Unknown import source %q", source)
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
		key := importers.ImportedKey{
			Name:       deriveImportedKeyName(path),
			PrivateKey: string(data),
		}
		if _, err := previewImportedKey(key); err != nil {
			continue
		}
		keys = append(keys, key)
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
		return nil, fmt.Errorf("Loading existing keys: %w", err)
	}

	var result struct {
		Keys []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("Parsing existing keys: %w", err)
	}

	seen := make(map[string]struct{}, len(result.Keys))
	for _, key := range result.Keys {
		if strings.TrimSpace(key.Fingerprint) != "" {
			seen[key.Fingerprint] = struct{}{}
		}
	}
	return seen, nil
}

func containsFingerprint(fingerprints map[string]struct{}, fingerprint string) bool {
	_, ok := fingerprints[fingerprint]
	return ok
}
