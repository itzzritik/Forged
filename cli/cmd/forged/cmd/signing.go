package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var signingCmd = &cobra.Command{
	Use:   "signing [key-name | off]",
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

		resp, err := client.Call("list", nil)
		if err != nil {
			return err
		}

		var result struct {
			Keys []struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				Fingerprint string `json:"fingerprint"`
			} `json:"keys"`
		}
		json.Unmarshal(resp.Data, &result)

		if len(result.Keys) == 0 {
			return fmt.Errorf("no keys in vault. Run: forged generate")
		}

		currentKey := getCurrentSigningKey()

		if currentKey != "" {
			matched := false
			for _, k := range result.Keys {
				pub := strings.Fields(k.Fingerprint)
				_ = pub
				// Match by checking if the current signing key contains any of our key fingerprints
				exportResp, _ := client.Call("export", map[string]string{"name": k.Name})
				if exportResp.Data != nil {
					var exp map[string]string
					json.Unmarshal(exportResp.Data, &exp)
					if strings.TrimSpace(exp["public_key"]) == strings.TrimSpace(currentKey) {
						fmt.Printf("  Current signing key: %s (%s)\n", k.Name, k.Fingerprint)
						matched = true
						break
					}
				}
			}
			if !matched {
				fmt.Printf("  Current signing key: external (not managed by Forged)\n")
				fmt.Printf("    %s\n", currentKey)
			}
			fmt.Println()
		} else {
			fmt.Println("  No signing key configured.\n")
		}

		// Filter out the current signing key
		var available []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Fingerprint string `json:"fingerprint"`
		}
		for _, k := range result.Keys {
			exportResp, _ := client.Call("export", map[string]string{"name": k.Name})
			if exportResp.Data != nil {
				var exp map[string]string
				json.Unmarshal(exportResp.Data, &exp)
				if strings.TrimSpace(exp["public_key"]) == strings.TrimSpace(currentKey) {
					continue
				}
			}
			available = append(available, k)
		}

		if len(available) == 0 {
			fmt.Println("  No other keys available. Generate a new key: forged generate")
			return nil
		}

		fmt.Println("  Switch signing key to:\n")
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

func enableSigning(client *ipc.Client, keyName string) error {
	resp, err := client.Call("export", map[string]string{"name": keyName})
	if err != nil {
		return err
	}

	var result map[string]string
	json.Unmarshal(resp.Data, &result)
	publicKey := result["public_key"]

	signPath, err := findSignBinary()
	if err != nil {
		return err
	}

	cmds := [][]string{
		{"git", "config", "--global", "user.signingkey", publicKey},
		{"git", "config", "--global", "gpg.format", "ssh"},
		{"git", "config", "--global", "gpg.ssh.program", signPath},
		{"git", "config", "--global", "commit.gpgsign", "true"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("running %v: %s: %w", args, string(out), err)
		}
	}

	writeAllowedSignersFile(publicKey)

	fmt.Printf("\n  Git signing enabled with key: %s\n", keyName)
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

func findSignBinary() (string, error) {
	path, err := exec.LookPath("forged-sign")
	if err == nil {
		return path, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find forged-sign binary")
	}
	candidate := filepath.Join(filepath.Dir(self), "forged-sign")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("forged-sign not found in PATH or next to forged binary")
}

func writeAllowedSignersFile(publicKey string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	signerFile := filepath.Join(home, ".ssh", "allowed_signers")

	if data, err := os.ReadFile(signerFile); err == nil {
		if strings.Contains(string(data), publicKey) {
			return
		}
	}

	os.MkdirAll(filepath.Dir(signerFile), 0700)
	f, err := os.OpenFile(signerFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "* %s\n", publicKey)
}

func init() {
	signingCmd.Flags().Bool("off", false, "disable Git commit signing")
	rootCmd.AddCommand(signingCmd)
}
