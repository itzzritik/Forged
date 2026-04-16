package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

type keyInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key,omitempty"`
}

var signingCmd = &cobra.Command{
	Use:   "signing [key-name]",
	Short: "Configure Git commit signing",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		off, _ := cmd.Flags().GetBool("off")
		if off {
			return disableSigning()
		}

		client := ctlClient()

		if len(args) == 1 {
			return enableSigning(client, args[0])
		}

		keys, err := listKeysWithPublicKeys(client)
		if err != nil {
			return err
		}

		if len(keys) == 0 {
			return fmt.Errorf("no keys in vault. Run: forged key generate")
		}

		currentKey := getCurrentSigningKey()
		currentKeyName := ""

		if currentKey != "" {
			for _, k := range keys {
				if strings.TrimSpace(k.PublicKey) == strings.TrimSpace(currentKey) {
					currentKeyName = k.Name
					fmt.Printf("  Current signing key: %s (%s)\n", k.Name, k.Fingerprint)
					break
				}
			}
			if currentKeyName == "" {
				fmt.Printf("  Current signing key: external (not managed by Forged)\n")
				fmt.Printf("    %s\n", currentKey)
			}
			fmt.Println()
		} else {
			fmt.Println("  No signing key configured.")
			fmt.Println()
		}

		var available []keyInfo
		for _, k := range keys {
			if k.Name != currentKeyName {
				available = append(available, k)
			}
		}

		if len(available) == 0 {
			fmt.Println("  No other keys available. Generate a new key: forged key generate")
			return nil
		}

		fmt.Println("  Switch signing key to:")
		fmt.Println()
		for i, k := range available {
			fmt.Printf("    %d. %s (%s)\n", i+1, k.Name, k.Fingerprint)
		}

		fmt.Println()
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("  Enter number (1-%d): ", len(available))
		line, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(line)

		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > len(available) {
			return fmt.Errorf("invalid selection")
		}

		return enableSigning(client, available[idx-1].Name)
	},
}

func listKeysWithPublicKeys(client *ipc.Client) ([]keyInfo, error) {
	resp, err := client.Call(ipc.CmdList, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Keys []keyInfo `json:"keys"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("parsing key list: %w", err)
	}

	for i, k := range result.Keys {
		if k.PublicKey == "" {
			exportResp, err := client.Call(ipc.CmdExport, map[string]string{"name": k.Name})
			if err == nil {
				var exp map[string]string
				json.Unmarshal(exportResp.Data, &exp)
				result.Keys[i].PublicKey = exp["public_key"]
			}
		}
	}

	return result.Keys, nil
}

func enableSigning(client *ipc.Client, keyName string) error {
	resp, err := client.Call(ipc.CmdExport, map[string]string{"name": keyName})
	if err != nil {
		return err
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("parsing export response: %w", err)
	}
	publicKey := result["public_key"]
	resolvedName := result["resolved_name"]
	if resolvedName == "" {
		resolvedName = keyName
	}

	signPath, err := findSignBinary()
	if err != nil {
		return err
	}

	if err := applyGitSigningConfig(publicKey, signPath); err != nil {
		return err
	}

	writeAllowedSigners(publicKey)

	fmt.Printf("\n  Git signing enabled with key: %s\n", resolvedName)
	fmt.Println("  All future commits will be signed automatically.")
	return nil
}

func disableSigning() error {
	cmds := [][]string{
		{"git", "config", "--global", "--unset", "user.signingkey"},
		{"git", "config", "--global", "--unset", "gpg.format"},
		{"git", "config", "--global", "--unset", "gpg.ssh.program"},
		{"git", "config", "--global", "--unset", "commit.gpgsign"},
	}
	for _, args := range cmds {
		exec.Command(args[0], args[1:]...).Run()
	}
	fmt.Println("  Git signing disabled.")
	return nil
}

func getCurrentSigningKey() string {
	out, err := exec.Command("git", "config", "--global", "user.signingkey").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func init() {
	signingCmd.Flags().Bool("off", false, "disable Git commit signing")
}
