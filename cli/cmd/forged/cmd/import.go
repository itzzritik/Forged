package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/itzzritik/forged/cli/internal/importers"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/vault"
	"github.com/spf13/cobra"
)

type importPreview struct {
	converted bool
	format    string
	key       importers.ImportedKey
	keyType   string
	rawFormat vault.PrivateKeyFormat
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

	previews, err := buildImportPreview(keys)
	if err != nil {
		return err
	}
	printImportPreview(previews)

	fmt.Println()
	fmt.Printf("  Import all %d key(s)? [Y/n] ", len(keys))
	confirm, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "n" || confirm == "no" {
		fmt.Println("  Aborted.")
		return nil
	}

	return doImport(keys)
}

func importFromSSHDir() error {
	paths := hostmatch.DiscoverSSHKeys()
	if len(paths) == 0 {
		fmt.Println("  No SSH keys found in ~/.ssh/")
		return nil
	}

	var keys []importers.ImportedKey
	var previews []importPreview
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
		preview, err := previewImportedKey(key)
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", p, err)
			continue
		}
		keys = append(keys, key)
		previews = append(previews, preview)
	}

	if len(keys) == 0 {
		fmt.Println("  No SSH keys found in ~/.ssh/")
		return nil
	}

	printImportPreview(previews)

	fmt.Println()
	fmt.Printf("  Import all %d key(s)? [Y/n] ", len(keys))
	confirm, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "n" || confirm == "no" {
		fmt.Println("  Aborted.")
		return nil
	}

	return doImport(keys)
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
	previews := make([]importPreview, 0, len(keys))
	for _, key := range keys {
		preview, err := previewImportedKey(key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key.Name, err)
		}
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
		key:       key,
		keyType:   normalizeImportedKeyType(normalized.Type),
		format:    privateKeyImportFormatLabel(normalized.Format),
		converted: normalized.Converted,
		rawFormat: normalized.Format,
	}
	return preview, nil
}

func printImportPreview(previews []importPreview) {
	fmt.Println()
	fmt.Printf("  Found %d SSH key(s):\n", len(previews))

	convertedFormats := make([]vault.PrivateKeyFormat, 0, len(previews))
	for i, preview := range previews {
		if preview.converted {
			convertedFormats = append(convertedFormats, preview.rawFormat)
		}
		fmt.Printf("    %d. %-20s (%s) [%s]\n", i+1, preview.key.Name, preview.keyType, preview.format)
	}

	warning, ok := privateKeyConversionSummary(convertedFormats)
	if !ok {
		return
	}

	fmt.Println()
	fmt.Println(warning)
}
