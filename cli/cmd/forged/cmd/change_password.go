package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	forgedsync "github.com/itzzritik/forged/cli/internal/sync"
	"github.com/itzzritik/forged/cli/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var changePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change vault master password",
	RunE: func(cmd *cobra.Command, args []string) error {
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return fmt.Errorf("change-password requires an interactive terminal")
		}

		fmt.Print("Current master password: ")
		currentPassword, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading current password: %w", err)
		}

		vaultPath := config.DefaultPaths().VaultFile()
		v, err := vault.Open(vaultPath, currentPassword)
		if err != nil {
			return fmt.Errorf("wrong password or corrupted vault")
		}
		defer v.Close()

		fmt.Print("New master password: ")
		newPassword, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading new password: %w", err)
		}

		fmt.Print("Confirm new password: ")
		confirmPassword, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}

		if string(newPassword) != string(confirmPassword) {
			return fmt.Errorf("passwords do not match")
		}

		if err := v.ChangePassword(newPassword); err != nil {
			return fmt.Errorf("changing password: %w", err)
		}

		creds, err := loadCredentials()
		if err != nil {
			fmt.Println("Password changed locally.")
			fmt.Println("Warning: Not synced to server (not logged in). Run 'forged login' then 'forged sync'.")
			return nil
		}

		client := forgedsync.NewClient(creds.ServerURL, creds.Token, "")

		err = client.Rekey(
			v.KDFParams(),
			base64.StdEncoding.EncodeToString(v.ProtectedKeyBytes()),
		)
		if err != nil {
			fmt.Println("Password changed locally.")
			fmt.Printf("Warning: Server sync failed: %s\n", err)
			fmt.Println("Run 'forged sync' to retry.")
			return nil
		}

		fmt.Println("Password changed successfully.")
		return nil
	},
}
