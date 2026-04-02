package cmd

import (
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock vault and clear keys from memory",
	RunE:  notImplemented("lock"),
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock vault",
	RunE:  notImplemented("unlock"),
}

var changePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change master password",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return fmt.Errorf("requires interactive terminal")
		}

		fmt.Fprint(os.Stderr, "Current password: ")
		oldPass, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stderr, "New password: ")
		newPass, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}

		if len(newPass) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}

		fmt.Fprint(os.Stderr, "Confirm new password: ")
		confirm, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}

		if string(newPass) != string(confirm) {
			return fmt.Errorf("passwords do not match")
		}

		v, err := vault.Open(paths.VaultFile(), oldPass)
		if err != nil {
			return fmt.Errorf("wrong current password")
		}
		defer v.Close()

		if err := v.ChangePassword(oldPass, newPass); err != nil {
			return err
		}

		fmt.Println("Password changed. Restart the daemon: forged stop && forged start")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(changePasswordCmd)
}
