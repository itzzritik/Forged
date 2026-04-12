package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	format              string
	key                 importers.ImportedKey
	keyType             string
	rawFormat           vault.PrivateKeyFormat
	selected            bool
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import SSH keys from 1Password, Bitwarden, or files",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().String("from", "", "import source: 1password, bitwarden, forged, ssh-dir, file")
	importCmd.Flags().String("file", "", "path to import file")

	rootCmd.AddCommand(importCmd)

	migrateAlias := &cobra.Command{
		Use:    "migrate",
		Hidden: true,
		RunE:   importCmd.RunE,
	}
	migrateAlias.Flags().String("from", "", "import source")
	migrateAlias.Flags().String("file", "", "path to import file")
	rootCmd.AddCommand(migrateAlias)
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
	}

	if from == "ssh-dir" {
		return importFromSSHDir()
	}

	if file == "" {
		fmt.Println()
		fmt.Print("  File path: ")
		line, _ := reader.ReadString('\n')
		file = strings.TrimSpace(line)
		if file == "" {
			return fmt.Errorf("file path is required")
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
		name := importers.SanitizeName(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
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

	return reviewAndImportKeys(reader, keys)
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

	return reviewAndImportKeys(bufio.NewReader(os.Stdin), keys)
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

	preview := importPreview{
		key:         key,
		keyType:     normalizeImportedKeyType(normalized.Type),
		format:      privateKeyImportFormatLabel(normalized.Format),
		converted:   normalized.Converted,
		fingerprint: normalized.Fingerprint,
		rawFormat:   normalized.Format,
	}
	return preview, nil
}

func printImportPreview(previews []importPreview) {
	fmt.Println()
	fmt.Printf("  Found %d SSH key(s):\n", len(previews))

	convertedFormats := make([]vault.PrivateKeyFormat, 0, len(previews))
	hasVaultDuplicates := hasImportVaultDuplicates(previews)
	collapsedDuplicateCount := 0
	for i, preview := range previews {
		if preview.converted {
			convertedFormats = append(convertedFormats, preview.rawFormat)
		}
		collapsedDuplicateCount += preview.collapsedDuplicates

		prefix := fmt.Sprintf("%d.", i+1)
		if hasVaultDuplicates {
			marker := " "
			if preview.selected {
				marker = "x"
			}
			prefix = fmt.Sprintf("%d. [%s]", i+1, marker)
		}

		fmt.Printf("    %-8s %-20s (%s) [%s]", prefix, preview.key.Name, preview.keyType, preview.format)
		if preview.alreadyInVault {
			fmt.Print(" [Already in Vault]")
		}
		fmt.Println()
		if preview.collapsedDuplicates > 0 {
			fmt.Printf("             %d duplicate entr%s consolidated from this import.\n", preview.collapsedDuplicates, pluralSuffix(preview.collapsedDuplicates, "y was", "ies were"))
		}
	}

	if hasVaultDuplicates {
		fmt.Println()
		fmt.Println("  Existing Keys Detected")
		fmt.Println("  Some imported keys already exist in this vault. Unique keys remain selected by default to prevent duplicate records.")
	}

	if collapsedDuplicateCount > 0 {
		fmt.Println()
		fmt.Println("  Duplicate entries in this import were consolidated by fingerprint before review.")
	}

	warning, ok := privateKeyConversionSummary(convertedFormats)
	if !ok {
		return
	}

	fmt.Println()
	fmt.Println(warning)
}

func reviewAndImportKeys(reader *bufio.Reader, keys []importers.ImportedKey) error {
	previews, err := buildImportPreview(keys)
	if err != nil {
		return err
	}

	printImportPreview(previews)

	if !hasImportVaultDuplicates(previews) {
		fmt.Println()
		fmt.Printf("  Import %d key(s)? [Y/n] ", len(previews))
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm == "n" || confirm == "no" {
			fmt.Println("  Aborted.")
			return nil
		}
		return doImport(selectedImportKeys(previews, false))
	}

	fmt.Println()
	fmt.Print("  Toggle any keys before import (comma-separated numbers, blank to continue): ")
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line != "" {
		if err := togglePreviewSelection(previews, line); err != nil {
			return err
		}
		fmt.Println()
		printImportPreview(previews)
	}

	fmt.Println()
	fmt.Println("  Choose import mode:")
	selectedLabel := "Import Selected Keys"
	if usesDefaultUniqueSelection(previews) {
		selectedLabel = "Import Unique Keys"
	}
	fmt.Printf("    1. %s\n", selectedLabel)
	fmt.Println("    2. Import All Keys")
	fmt.Println("    3. Cancel")
	fmt.Println()
	fmt.Print("  Choice [1-3]: ")

	choice, _ := reader.ReadString('\n')
	switch strings.TrimSpace(choice) {
	case "1", "":
		selected := selectedImportKeys(previews, false)
		if len(selected) == 0 {
			fmt.Println("  No keys selected. Aborted.")
			return nil
		}
		return doImport(selected)
	case "2":
		return doImport(selectedImportKeys(previews, true))
	case "3":
		fmt.Println("  Aborted.")
		return nil
	default:
		return fmt.Errorf("invalid choice")
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

func hasImportVaultDuplicates(previews []importPreview) bool {
	for _, preview := range previews {
		if preview.alreadyInVault {
			return true
		}
	}
	return false
}

func usesDefaultUniqueSelection(previews []importPreview) bool {
	for _, preview := range previews {
		if preview.selected != !preview.alreadyInVault {
			return false
		}
	}
	return true
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

func togglePreviewSelection(previews []importPreview, input string) error {
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		index, err := strconv.Atoi(part)
		if err != nil || index < 1 || index > len(previews) {
			return fmt.Errorf("invalid selection %q", part)
		}
		previews[index-1].selected = !previews[index-1].selected
	}
	return nil
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
