package cmd

import (
	"bufio"
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
	alreadyInVault bool
	converted      bool
	fingerprint    string
	key            importers.ImportedKey
	selected       bool
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import SSH keys from 1Password, Bitwarden, or files",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().String("from", "", "import source: 1password, bitwarden, forged, ssh-dir, file")
	importCmd.Flags().String("file", "", "path to import file")
}

func runImport(cmd *cobra.Command, args []string) error {
	from, _ := cmd.Flags().GetString("from")
	file, _ := cmd.Flags().GetString("file")

	reader := bufio.NewReader(os.Stdin)

	if from == "" {
		fmt.Println()
		fmt.Println("  Select a source:")
		fmt.Println()
		fmt.Println("    1. 1Password (.1pux, .csv)")
		fmt.Println("    2. Bitwarden (.json)")
		fmt.Println("    3. Forged export (.json)")
		fmt.Println("    4. SSH directory (~/.ssh/)")
		fmt.Println("    5. SSH key file")
		fmt.Println()
		fmt.Print("  Choice [1-5]: ")

		line, _ := reader.ReadString('\n')
		switch strings.TrimSpace(line) {
		case "1":
			from = "1password"
		case "2":
			from = "bitwarden"
		case "3":
			from = "forged"
		case "4":
			from = "ssh-dir"
		case "5":
			from = "file"
		default:
			return fmt.Errorf("invalid choice")
		}
		printStepSeparator()
	}

	if from == "ssh-dir" {
		return importFromSSHDir()
	}

	if file == "" {
		if terminalIsInteractive() {
			if picked, ok := chooseFileWithPicker(); ok {
				file = picked
			}
		}
		if file == "" {
			fmt.Println()
			fmt.Print("  File path: ")
			line, _ := reader.ReadString('\n')
			file = strings.TrimSpace(line)
			if file == "" {
				return fmt.Errorf("file path is required")
			}
			printStepSeparator()
		}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
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
		return fmt.Errorf("unknown source: %s", from)
	}
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println("  No SSH keys found.")
		return nil
	}

	return reviewAndImportKeys(reader, importSourceLabel(from), keys)
}

func importFromSSHDir() error {
	paths := hostmatch.DiscoverSSHKeys()
	if len(paths) == 0 {
		fmt.Println("  No SSH keys found in ~/.ssh/")
		return nil
	}

	var keys []importers.ImportedKey
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", p, err)
			continue
		}
		key := importers.ImportedKey{
			Name:       deriveKeyName(p),
			PrivateKey: string(data),
		}
		if _, err := previewImportedKey(key); err != nil {
			fmt.Printf("  Skipped %s: %v\n", p, err)
			continue
		}
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		fmt.Println("  No SSH keys found in ~/.ssh/")
		return nil
	}

	return reviewAndImportKeys(bufio.NewReader(os.Stdin), importSourceLabel("ssh-dir"), keys)
}

func doImport(keys []importers.ImportedKey) error {
	client := ctlClient()
	imported := 0

	for _, k := range keys {
		_, err := client.Call(ipc.CmdAdd, map[string]string{
			"name":        k.Name,
			"private_key": k.PrivateKey,
			"comment":     "",
		})
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", k.Name, err)
			continue
		}
		fmt.Printf("  Imported %s\n", k.Name)
		imported++
	}

	fmt.Println()
	fmt.Printf("  %d key(s) imported.\n", imported)
	return nil
}

func buildImportPreview(keys []importers.ImportedKey) ([]importPreview, error) {
	existingFingerprints, err := loadExistingVaultFingerprints()
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
		if _, ok := byFingerprint[preview.fingerprint]; ok {
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

func reviewAndImportKeys(reader *bufio.Reader, sourceLabel string, keys []importers.ImportedKey) error {
	previews, err := buildImportPreview(keys)
	if err != nil {
		return err
	}

	state := newImportReviewState(sourceLabel, previews)
	return runImportReview(reader, state)
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

func selectedImportKeys(previews []importPreview, includeAll bool) []importers.ImportedKey {
	keys := make([]importers.ImportedKey, 0, len(previews))
	for _, preview := range previews {
		if includeAll || preview.selected {
			keys = append(keys, preview.key)
		}
	}
	return keys
}
