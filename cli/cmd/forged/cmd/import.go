package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/itzzritik/forged/cli/internal/importers"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/vault"
	"github.com/spf13/cobra"
)

type importPreview struct {
	alreadyInVault      bool
	collapsedDuplicates int
	converted           bool
	fingerprint         string
	key                 importers.ImportedKey
	selected            bool
}

type importMode string

const (
	importModeTUI      importMode = "tui"
	importModeScripted importMode = "scripted"
	importModeInvalid  importMode = "invalid"
)

var loadExistingVaultFingerprintsFunc = loadExistingVaultFingerprints

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import SSH keys from 1Password, Bitwarden, or files",
	Example: strings.TrimSpace(`
  forged key import
  forged key import --from ssh-dir
  forged key import --from file --file ~/.ssh/id_ed25519
	`),
	RunE: runImport,
}

func init() {
	importCmd.Flags().String("from", "", "import source: 1password, bitwarden, forged, ssh-dir, file")
	importCmd.Flags().String("file", "", "path to import file")
}

func determineImportMode(interactive bool, from, file string) importMode {
	if interactive {
		return importModeTUI
	}
	if from != "" || file != "" {
		return importModeScripted
	}
	return importModeInvalid
}

func runImport(cmd *cobra.Command, args []string) error {
	from, _ := cmd.Flags().GetString("from")
	file, _ := cmd.Flags().GetString("file")

	switch determineImportMode(terminalIsInteractive() && !jsonOutput, from, file) {
	case importModeTUI:
		return runImportTUI(cmd, from, file, false)
	case importModeScripted:
		return runImportScripted(from, file)
	default:
		return fmt.Errorf("forged key import requires an interactive terminal or both --from / --file in scripted use")
	}
}

func runImportScripted(from, file string) error {
	keys, _, err := loadImportedKeys(from, file)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Println("  No SSH keys found.")
		return nil
	}
	return doImport(keys)
}

func loadImportedKeys(from, file string) ([]importers.ImportedKey, string, error) {
	sourceLabel := importSourceLabel(from)
	if from == "" {
		return nil, "", fmt.Errorf("import source is required")
	}
	if from == "ssh-dir" {
		keys, err := importFromSSHDir()
		return keys, sourceLabel, err
	}
	if file == "" {
		return nil, sourceLabel, fmt.Errorf("file path is required")
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, sourceLabel, fmt.Errorf("reading file: %w", err)
	}

	var keys []importers.ImportedKey
	switch from {
	case "1password":
		keys, err = importers.Parse1Password(data)
	case "bitwarden":
		keys, err = importers.ParseBitwarden(data)
	case "forged":
		keys, err = importers.ParseForged(data)
	case "file":
		name := deriveKeyName(file)
		keys = []importers.ImportedKey{{Name: name, PrivateKey: string(data)}}
	default:
		return nil, sourceLabel, fmt.Errorf("unknown source: %s", from)
	}
	if err != nil {
		return nil, sourceLabel, fmt.Errorf("parsing file: %w", err)
	}

	return keys, sourceLabel, nil
}

func importFromSSHDir() ([]importers.ImportedKey, error) {
	paths := hostmatch.DiscoverSSHKeys()
	if len(paths) == 0 {
		return nil, nil
	}

	var keys []importers.ImportedKey
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		key := importers.ImportedKey{
			Name:       deriveKeyName(p),
			PrivateKey: string(data),
		}
		if _, err := previewImportedKey(key); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	return keys, nil
}

type importExecutionResult struct {
	Imported int
	Skipped  int
}

func executeImport(keys []importers.ImportedKey) (importExecutionResult, error) {
	client := ctlClient()
	result := importExecutionResult{}

	for _, k := range keys {
		_, err := client.Call(ipc.CmdAdd, map[string]string{
			"name":        k.Name,
			"private_key": k.PrivateKey,
			"comment":     "",
		})
		if err != nil {
			result.Skipped++
			continue
		}
		result.Imported++
	}

	return result, nil
}

func doImport(keys []importers.ImportedKey) error {
	result, err := executeImport(keys)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  %d key(s) imported.\n", result.Imported)
	if result.Skipped > 0 {
		fmt.Printf("  %d key(s) skipped.\n", result.Skipped)
	}
	return nil
}

func buildImportPreview(keys []importers.ImportedKey) ([]importPreview, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	existingFingerprints, err := loadExistingVaultFingerprintsFunc()
	if err != nil {
		return nil, err
	}

	previews := make([]importPreview, 0, len(keys))
	byFingerprint := make(map[string]int, len(keys))
	for _, key := range keys {
		preview, err := previewImportedKey(key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key.Name, err)
		}
		if idx, ok := byFingerprint[preview.fingerprint]; ok {
			previews[idx].collapsedDuplicates++
			continue
		}
		preview.alreadyInVault = containsFingerprint(existingFingerprints, preview.fingerprint)
		preview.selected = !preview.alreadyInVault
		byFingerprint[preview.fingerprint] = len(previews)
		previews = append(previews, preview)
	}
	return previews, nil
}

func previewImportedKey(key importers.ImportedKey) (importPreview, error) {
	normalized, err := vault.NormalizePrivateKeyToOpenSSH([]byte(key.PrivateKey), "")
	if err != nil {
		return importPreview{}, formatPrivateKeyImportError(err)
	}

	return importPreview{
		key:         key,
		converted:   normalized.Converted,
		fingerprint: normalized.Fingerprint,
	}, nil
}

func importSourceLabel(from string) string {
	switch from {
	case "1password":
		return "1Password import"
	case "bitwarden":
		return "Bitwarden import"
	case "forged":
		return "Forged export import"
	case "ssh-dir":
		return "SSH directory import"
	case "file":
		return "SSH key file import"
	default:
		return "Key import"
	}
}

func loadExistingVaultFingerprints() (map[string]struct{}, error) {
	resp, err := ctlClient().Call(ipc.CmdList, nil)
	if err != nil {
		return nil, fmt.Errorf("loading existing keys: %w", err)
	}

	var result struct {
		Keys []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing key list: %w", err)
	}

	fingerprints := make(map[string]struct{}, len(result.Keys))
	for _, key := range result.Keys {
		if key.Fingerprint == "" {
			continue
		}
		fingerprints[key.Fingerprint] = struct{}{}
	}
	return fingerprints, nil
}

func containsFingerprint(fingerprints map[string]struct{}, fingerprint string) bool {
	_, ok := fingerprints[fingerprint]
	return ok
}
