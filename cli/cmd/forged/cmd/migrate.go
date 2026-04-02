package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import keys from ~/.ssh/, 1Password, or ssh-agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		source, _ := cmd.Flags().GetString("from")

		switch source {
		case "ssh":
			return migrateFromSSH()
		case "1password":
			return migrateFrom1Password()
		case "agent":
			return migrateFromAgent()
		default:
			return migrateFromSSH()
		}
	},
}

func init() {
	migrateCmd.Flags().String("from", "ssh", "import source: ssh, 1password, agent")

	rootCmd.AddCommand(migrateCmd)
}

func migrateFromSSH() error {
	keys := hostmatch.DiscoverSSHKeys()
	if len(keys) == 0 {
		fmt.Println("No SSH keys found in ~/.ssh/")
		return nil
	}

	fmt.Printf("Found %d key(s):\n", len(keys))
	for i, p := range keys {
		fmt.Printf("  %d. %s\n", i+1, p)
	}

	client := ctlClient()

	imported := 0
	for _, keyPath := range keys {
		name := deriveKeyNameFromPath(keyPath)

		data, err := os.ReadFile(keyPath)
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", keyPath, err)
			continue
		}

		_, err = client.Call("add", map[string]string{
			"name":        name,
			"private_key": string(data),
			"comment":     "",
		})
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", filepath.Base(keyPath), err)
			continue
		}

		fmt.Printf("  Imported %s as %q\n", filepath.Base(keyPath), name)
		imported++
	}

	fmt.Printf("\n%d key(s) imported\n", imported)
	return nil
}

func migrateFrom1Password() error {
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("1Password CLI (op) not found. Install it from https://1password.com/downloads/command-line/")
	}

	out, err := exec.Command("op", "item", "list", "--categories", "SSH Key", "--format", "json").Output()
	if err != nil {
		return fmt.Errorf("listing 1Password SSH keys: %w (are you signed in? Run: op signin)", err)
	}

	var items []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return fmt.Errorf("parsing 1Password output: %w", err)
	}

	if len(items) == 0 {
		fmt.Println("No SSH keys found in 1Password")
		return nil
	}

	fmt.Printf("Found %d SSH key(s) in 1Password:\n", len(items))
	for i, item := range items {
		fmt.Printf("  %d. %s\n", i+1, item.Title)
	}

	client := ctlClient()

	imported := 0
	for _, item := range items {
		privKey, err := exec.Command("op", "item", "get", item.ID, "--fields", "private key", "--reveal").Output()
		if err != nil {
			fmt.Printf("  Skipped %s: could not read private key\n", item.Title)
			continue
		}

		name := sanitizeName(item.Title)
		_, err = client.Call("add", map[string]string{
			"name":        name,
			"private_key": string(privKey),
			"comment":     "imported from 1Password",
		})
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", item.Title, err)
			continue
		}

		fmt.Printf("  Imported %s as %q\n", item.Title, name)
		imported++
	}

	fmt.Printf("\n%d key(s) imported\n", imported)
	return nil
}

func migrateFromAgent() error {
	fmt.Println("Listing keys from current SSH agent...")
	fmt.Println("Note: ssh-agent only exposes public keys. You'll need to provide the private key files.")

	out, err := exec.Command("ssh-add", "-L").Output()
	if err != nil {
		return fmt.Errorf("could not list agent keys: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		fmt.Println("No keys in current SSH agent")
		return nil
	}

	fmt.Printf("Found %d key(s) in agent:\n", len(lines))
	for i, line := range lines {
		parts := strings.Fields(line)
		comment := ""
		if len(parts) >= 3 {
			comment = parts[2]
		}
		fmt.Printf("  %d. %s %s\n", i+1, parts[0], comment)
	}

	fmt.Println("\nTo import these keys, use: forged migrate --from ssh")
	fmt.Println("(The agent protocol only exposes public keys, not private keys)")
	return nil
}

func deriveKeyNameFromPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".pem")
	name = strings.TrimPrefix(name, "id_")
	if name == "" {
		name = "default"
	}
	return name
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, name)
	if name == "" {
		name = "imported"
	}
	return name
}
