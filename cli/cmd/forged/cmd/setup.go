package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/hostmatch"
	"github.com/itzzritik/forged/cli/internal/vault"
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

		if err := config.EnableSSHAgent(paths); err != nil {
			fmt.Printf("Warning: could not configure Forged SSH include: %v\n", err)
		}

		keys := ks.List()
		if len(keys) > 0 {
			fmt.Print("\nSet up Git commit signing? [Y/n] ")
			if confirm() {
				if err := configureGitSigning(ks, keys[0].Name); err != nil {
					fmt.Printf("Warning: could not configure git signing: %v\n", err)
				}
			}
		}

		fmt.Println("\nInstalling daemon service...")
		if err := daemon.InstallService(paths, string(password)); err != nil {
			fmt.Printf("Warning: could not install service: %v\n", err)
			fmt.Println("You can start manually with: forged daemon")
		} else {
			if err := daemon.StartService(); err != nil {
				fmt.Printf("Warning: could not start service: %v\n", err)
				fmt.Println("Start manually with: forged start")
			} else {
				time.Sleep(2 * time.Second)
				if pid, running := daemon.IsRunning(paths); running {
					fmt.Printf("Daemon running (PID %d)\n", pid)
				}
			}
		}

		fmt.Println()
		fmt.Println("Setup complete!")
		fmt.Printf("  Vault:        %s\n", paths.VaultFile())
		fmt.Printf("  Config:       %s\n", paths.ConfigFile())
		fmt.Printf("  Agent:        %s\n", paths.AgentSocket())
		fmt.Printf("  SSH include:  %s\n", paths.SSHBaseInclude())
		fmt.Printf("  SSH routes:   %s\n", paths.SSHAdvancedConfig())
		fmt.Println("  Advanced provider routing is generated only when Forged needs it.")
		return nil
	},
}

func createPassword() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("setup requires an interactive terminal")
	}

	for {
		fmt.Print("Create a master password: ")
		pass1, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return nil, err
		}

		if len(pass1) < 8 {
			fmt.Println("  Password must be at least 8 characters. Try again.\n")
			continue
		}

		fmt.Print("Confirm: ")
		pass2, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return nil, err
		}

		if string(pass1) != string(pass2) {
			fmt.Println("  Passwords do not match. Try again.\n")
			continue
		}

		return pass1, nil
	}
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

func configureGitSigning(ks *vault.KeyStore, keyName string) error {
	key, ok := ks.Get(keyName)
	if !ok {
		return fmt.Errorf("key %q not found", keyName)
	}

	if err := ks.SetGitSigning(keyName, true); err != nil {
		return err
	}

	signPath, err := findSignBinary()
	if err != nil {
		return err
	}

	if err := applyGitSigningConfig(key.PublicKey, signPath); err != nil {
		return err
	}

	if err := writeAllowedSigners(key.PublicKey); err != nil {
		return err
	}

	fmt.Printf("  Git signing configured with %s\n", keyName)
	return nil
}
