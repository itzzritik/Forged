package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
	"github.com/itzzritik/forged/cli/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func ctlClient() *ipc.Client {
	return ipc.NewClient(config.DefaultPaths().CtlSocket())
}

var addCmd = &cobra.Command{
	Use:   "add <name> --file <path>",
	Short: "Import a key from file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading key file: %w", err)
		}
		comment, _ := cmd.Flags().GetString("comment")
		normalized, normErr := vault.NormalizePrivateKeyToOpenSSH(data, comment)
		if normErr != nil {
			return formatPrivateKeyImportError(normErr)
		}
		if normalized.Converted {
			fmt.Println(singlePrivateKeyConversionWarning(normalized.Format))
			fmt.Println()
		}

		resp, err := ctlClient().Call(ipc.CmdAdd, map[string]string{
			"name":        args[0],
			"private_key": string(data),
			"comment":     comment,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result map[string]string
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
		fmt.Printf("Added %s (%s)\n  %s\n", result["name"], result["type"], result["fingerprint"])
		if normalized.Converted {
			fmt.Println("  Stored as canonical OpenSSH private key format.")
		}
		return nil
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate [name]",
	Short: "Generate a new Ed25519 key pair",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		comment, _ := cmd.Flags().GetString("comment")

		if name == "" && !jsonOutput {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("  A short name to identify this key (e.g. github, work, prod-server)")
			fmt.Print("  Name: ")
			line, _ := reader.ReadString('\n')
			name = strings.TrimSpace(line)
			if name == "" {
				return fmt.Errorf("key name is required")
			}

			if comment == "" {
				fmt.Println()
				fmt.Println("  A label attached to the public key (e.g. your email or username)")
				fmt.Print("  Label: ")
				line, _ = reader.ReadString('\n')
				comment = strings.TrimSpace(line)
			}
		}

		if name == "" {
			return fmt.Errorf("key name is required")
		}

		resp, err := ctlClient().Call(ipc.CmdGenerate, map[string]string{
			"name":    name,
			"comment": comment,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result map[string]string
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		pub := result["public_key"]
		if comment != "" {
			pub = pub + " " + comment
		}

		fmt.Println()
		fmt.Printf("  Key:         %s\n", result["name"])
		fmt.Printf("  Type:        %s\n", result["type"])
		fmt.Printf("  Fingerprint: %s\n", result["fingerprint"])
		fmt.Println()
		fmt.Println("  Public key (add this to GitHub/GitLab/Server):")
		fmt.Println()
		fmt.Printf("    %s\n", pub)
		fmt.Println()

		fmt.Println("  Add this public key to:")
		fmt.Println("    GitHub:  Settings > SSH Keys > New SSH Key")
		fmt.Println("    GitLab:  Preferences > SSH Keys > Add new key")
		fmt.Println("    Server:  ssh-copy-id or append to ~/.ssh/authorized_keys")
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call(ipc.CmdList, nil)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result struct {
			Keys []struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				Fingerprint string `json:"fingerprint"`
			} `json:"keys"`
		}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}

		if len(result.Keys) == 0 {
			fmt.Println("No keys in vault")
			return nil
		}

		signingKey := getCurrentSigningKey()

		type row struct {
			name, keyType, fingerprint, signing string
		}
		var rows []row
		colW := [4]int{4, 4, 11, 7} // NAME, TYPE, FINGERPRINT, SIGNING

		for _, k := range result.Keys {
			r := row{name: k.Name, keyType: k.Type, fingerprint: k.Fingerprint}
			exportResp, _ := ctlClient().Call(ipc.CmdExport, map[string]string{"name": k.Name})
			if exportResp.Data != nil {
				var exp map[string]string
				json.Unmarshal(exportResp.Data, &exp)
				if strings.TrimSpace(exp["public_key"]) == strings.TrimSpace(signingKey) {
					r.signing = "yes"
				}
			}
			rows = append(rows, r)
			if len(r.name) > colW[0] {
				colW[0] = len(r.name)
			}
			if len(r.keyType) > colW[1] {
				colW[1] = len(r.keyType)
			}
			if len(r.fingerprint) > colW[2] {
				colW[2] = len(r.fingerprint)
			}
		}

		header := fmt.Sprintf("  %-*s  %-*s  %-*s  %s", colW[0], "NAME", colW[1], "TYPE", colW[2], "FINGERPRINT", "SIGNING")
		divider := "  " + strings.Repeat("-", colW[0]) + "  " + strings.Repeat("-", colW[1]) + "  " + strings.Repeat("-", colW[2]) + "  " + strings.Repeat("-", 7)

		fmt.Println(header)
		fmt.Println(divider)
		for _, r := range rows {
			fmt.Printf("  %-*s  %-*s  %-*s  %s\n", colW[0], r.name, colW[1], r.keyType, colW[2], r.fingerprint, r.signing)
		}
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call(ipc.CmdRemove, map[string]string{"name": args[0]})
		if err != nil {
			return err
		}
		var result map[string]string
		_ = json.Unmarshal(resp.Data, &result)
		name := result["resolved_name"]
		if name == "" {
			name = args[0]
		}
		fmt.Printf("Removed %s\n", name)
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the full vault to a Forged JSON file",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("forged export no longer accepts a key name.\nUse `forged view <name>` to inspect a key, or `forged export` to export the full vault")
		}
		exportAll, _ := cmd.Flags().GetBool("all")
		if exportAll {
			return fmt.Errorf("forged export no longer accepts --all.\nUse `forged export` to export the full vault")
		}
		return exportVault(cmd)
	},
}

var viewCmd = &cobra.Command{
	Use:   "view <name>",
	Short: "View key details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		return viewKey(args[0], full)
	},
}

type viewResult struct {
	ResolvedName string           `json:"resolved_name"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Fingerprint  string           `json:"fingerprint"`
	PublicKey    string           `json:"public_key"`
	PrivateKey   string           `json:"private_key,omitempty"`
	Comment      string           `json:"comment,omitempty"`
	HostRules    []vault.HostRule `json:"host_rules"`
	CreatedAt    string           `json:"created_at,omitempty"`
	UpdatedAt    string           `json:"updated_at,omitempty"`
	LastUsedAt   string           `json:"last_used_at,omitempty"`
	Version      int              `json:"version,omitempty"`
	DeviceOrigin string           `json:"device_origin,omitempty"`
	GitSigning   bool             `json:"git_signing,omitempty"`
}

type exportedKey struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	GitSigning  bool   `json:"git_signing"`
	HostRules   any    `json:"host_rules"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func exportVault(cmd *cobra.Command) error {
	outPath, _ := cmd.Flags().GetString("out")
	defaultName := fmt.Sprintf("forged-export-%s.json", time.Now().Format("2006-01-02"))
	if outPath == "" {
		if !terminalIsInteractive() {
			return fmt.Errorf("forged export requires --out when not interactive")
		}
		if selection, ok := chooseSavePathWithPicker(defaultName); ok {
			outPath = selection
		} else {
			var err error
			outPath, err = promptForSavePath(defaultName)
			if err != nil {
				return fmt.Errorf("reading save path: %w", err)
			}
		}
		printStepSeparator()
	}

	authResult, err := authorizeSensitiveAction(sensitiveauth.ActionExport)
	if err != nil {
		return err
	}
	if authResult.ExportToken == "" {
		return fmt.Errorf("export authorization did not return a token")
	}

	resp, err := ctlClient().Call(ipc.CmdExportAll, map[string]string{"token": authResult.ExportToken})
	if err != nil {
		return err
	}

	var keys []exportedKey
	if err := json.Unmarshal(resp.Data, &keys); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	items := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		items = append(items, map[string]any{
			"type": "ssh_key",
			"name": k.Name,
			"ssh_key": map[string]any{
				"private_key": k.PrivateKey,
				"public_key":  k.PublicKey,
				"fingerprint": k.Fingerprint,
				"key_type":    k.Type,
				"comment":     k.Comment,
				"host_rules":  k.HostRules,
				"git_signing": k.GitSigning,
			},
			"created_at": k.CreatedAt,
			"updated_at": k.UpdatedAt,
		})
	}

	export := map[string]any{
		"format":      "forged-export",
		"version":     1,
		"exported_at": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"items":       items,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling export: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0600); err != nil {
		return fmt.Errorf("writing export file: %w", err)
	}

	if jsonOutput {
		return printOutput(map[string]any{
			"path":      outPath,
			"key_count": len(keys),
		})
	}

	fmt.Printf("Exported %d keys to %s\n", len(keys), outPath)
	return nil
}

func viewKey(name string, full bool) error {
	if full {
		if _, err := authorizeSensitiveAction(sensitiveauth.ActionView); err != nil {
			return err
		}
	}

	resp, err := ctlClient().Call(ipc.CmdView, map[string]any{
		"name": name,
		"full": full,
	})
	if err != nil {
		return err
	}
	if jsonOutput {
		return printOutput(json.RawMessage(resp.Data))
	}

	var result viewResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	printViewDetailBlock(result, full)
	return nil
}

func authorizeSensitiveAction(action sensitiveauth.Action) (sensitiveauth.AuthorizeResult, error) {
	client := ctlClient()

	parseResult := func(raw json.RawMessage) (sensitiveauth.AuthorizeResult, error) {
		var result sensitiveauth.AuthorizeResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return sensitiveauth.AuthorizeResult{}, fmt.Errorf("parsing auth response: %w", err)
		}
		return result, nil
	}

	resp, err := client.CallWithTimeout(ipc.CmdSensitiveAuth, map[string]string{
		"action": string(action),
	}, 5*time.Minute)
	if err != nil {
		return sensitiveauth.AuthorizeResult{}, err
	}

	result, err := parseResult(resp.Data)
	if err != nil {
		return sensitiveauth.AuthorizeResult{}, err
	}
	if !result.PasswordRequired {
		return result, nil
	}
	if !terminalIsInteractive() {
		return sensitiveauth.AuthorizeResult{}, fmt.Errorf("sensitive private-key access requires interactive authentication")
	}

	password, err := readSensitivePassword(result.Prompt)
	if err != nil {
		return sensitiveauth.AuthorizeResult{}, err
	}
	defer zeroPassword(password)

	resp, err = client.CallWithTimeout(ipc.CmdSensitivePassword, map[string]string{
		"action":   string(action),
		"password": string(password),
	}, 5*time.Minute)
	if err != nil {
		return sensitiveauth.AuthorizeResult{}, err
	}

	return parseResult(resp.Data)
}

func readSensitivePassword(prompt string) ([]byte, error) {
	if prompt == "" {
		prompt = "Enter your master password to continue:"
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("sensitive private-key access requires interactive authentication")
	}

	fmt.Fprint(os.Stderr, prompt+" ")
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	return password, nil
}

func zeroPassword(password []byte) {
	for i := range password {
		password[i] = 0
	}
}

func printViewDetailBlock(result viewResult, full bool) {
	fmt.Printf("  %s\n", result.Name)
	fmt.Printf("  %s\n", result.Type)
	fmt.Printf("  %s\n", result.Fingerprint)
	fmt.Println()

	fmt.Println("  Public key")
	fmt.Printf("    %s\n", result.PublicKey)
	fmt.Println()

	if full && result.PrivateKey != "" {
		fmt.Println("  Private key")
		for _, line := range strings.Split(strings.TrimRight(result.PrivateKey, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
		fmt.Println()
	}

	fmt.Println("  Host rules")
	if len(result.HostRules) == 0 {
		fmt.Println("    None")
	} else {
		for _, rule := range result.HostRules {
			fmt.Printf("    %s\n", rule.Match)
		}
	}
	fmt.Println()

	fmt.Println("  Metadata")
	if result.CreatedAt != "" {
		fmt.Printf("    Created:   %s\n", formatViewTimestamp(result.CreatedAt))
	}
	if result.UpdatedAt != "" {
		fmt.Printf("    Updated:   %s\n", formatViewTimestamp(result.UpdatedAt))
	}
	if result.LastUsedAt != "" {
		fmt.Printf("    Last used: %s\n", formatViewTimestamp(result.LastUsedAt))
	}
	if result.Version > 0 {
		fmt.Printf("    Version:   %d\n", result.Version)
	}
	if result.DeviceOrigin != "" {
		fmt.Printf("    Device:    %s\n", result.DeviceOrigin)
	}
}

func formatViewTimestamp(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call(ipc.CmdRename, map[string]string{
			"old_name": args[0],
			"new_name": args[1],
		})
		if err != nil {
			return err
		}
		var result map[string]string
		_ = json.Unmarshal(resp.Data, &result)
		oldName := result["old_name"]
		if oldName == "" {
			oldName = args[0]
		}
		fmt.Printf("Renamed %s → %s\n", oldName, args[1])
		return nil
	},
}

func init() {
	addCmd.Flags().StringP("file", "f", "", "path to private key file")
	addCmd.Flags().StringP("comment", "c", "", "key comment")
	generateCmd.Flags().StringP("comment", "c", "", "key comment")
	exportCmd.Flags().Bool("all", false, "legacy full-vault export flag")
	exportCmd.Flags().StringP("out", "o", "", "output file path")
	_ = exportCmd.Flags().MarkHidden("all")
	viewCmd.Flags().Bool("full", false, "include the private key after authentication")
}
