package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forgedkeys/forged/cli/internal/config"
	"github.com/forgedkeys/forged/cli/internal/hostmatch"
	"github.com/forgedkeys/forged/cli/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if _, err := os.Stat(paths.VaultFile()); err == nil {
			return fmt.Errorf("vault already exists at %s. Delete it to re-run setup", paths.VaultFile())
		}

		fmt.Println("Welcome to Forged! Let's set up your SSH key manager.")
		fmt.Println()

		password, err := createPassword()
		if err != nil {
			return err
		}

		v, err := vault.Create(paths.VaultFile(), password)
		if err != nil {
			return fmt.Errorf("creating vault: %w", err)
		}
		defer v.Close()

		ks := vault.NewKeyStore(v)

		discovered := hostmatch.DiscoverSSHKeys()
		if len(discovered) > 0 {
			fmt.Printf("\nFound %d SSH key(s):\n", len(discovered))
			for i, p := range discovered {
				fmt.Printf("  %d. %s\n", i+1, p)
			}
			fmt.Print("\nImport these keys? [Y/n] ")
			if confirm() {
				importKeys(ks, discovered)
			}
		}

		if err := writeDefaultConfig(paths); err != nil {
			fmt.Printf("Warning: could not write config: %v\n", err)
		}

		if err := injectSSHConfig(paths); err != nil {
			fmt.Printf("Warning: could not update ~/.ssh/config: %v\n", err)
		}

		fmt.Println()
		fmt.Println("Setup complete!")
		fmt.Printf("  Vault:  %s\n", paths.VaultFile())
		fmt.Printf("  Config: %s\n", paths.ConfigFile())
		fmt.Printf("  Socket: %s\n", paths.AgentSocket())
		fmt.Println()
		fmt.Println("Start the daemon with: forged daemon")
		return nil
	},
}

func createPassword() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("setup requires an interactive terminal")
	}

	fmt.Print("Create a master password: ")
	pass1, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return nil, err
	}

	if len(pass1) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	fmt.Print("Confirm: ")
	pass2, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return nil, err
	}

	if string(pass1) != string(pass2) {
		return nil, fmt.Errorf("passwords do not match")
	}

	return pass1, nil
}

func confirm() bool {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}

func importKeys(ks *vault.KeyStore, paths []string) {
	for _, p := range paths {
		name := deriveKeyName(p)
		_, err := ks.AddFromFile(name, p, "")
		if err != nil {
			fmt.Printf("  Skipped %s: %v\n", p, err)
			continue
		}
		fmt.Printf("  Imported %s as %q\n", filepath.Base(p), name)
	}
}

func deriveKeyName(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".pem")
	name = strings.TrimPrefix(name, "id_")
	if name == "" {
		name = "default"
	}
	return name
}

func writeDefaultConfig(paths config.Paths) error {
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(paths.ConfigFile()); err == nil {
		return nil
	}

	content := fmt.Sprintf(`[agent]
socket = %q
log_level = "info"

[sync]
enabled = false
`, paths.AgentSocket())

	return os.WriteFile(paths.ConfigFile(), []byte(content), 0600)
}

func injectSSHConfig(paths config.Paths) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	marker := "# Added by Forged"
	directive := fmt.Sprintf("Host *\n    IdentityAgent %q\n", paths.AgentSocket())

	if data, err := os.ReadFile(sshConfigPath); err == nil {
		if strings.Contains(string(data), "IdentityAgent") && strings.Contains(string(data), "forged") {
			return nil
		}
		if strings.Contains(string(data), marker) {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "\n%s\n%s", marker, directive)
	return err
}
