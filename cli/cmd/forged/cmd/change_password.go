package cmd

import (
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var changePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change vault master password",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !jsonOutput && isInteractiveTerminal() {
			return runInteractiveIntent(tui.ResolveCommand([]string{"vault", "change-password"}, args))
		}

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

		result, err := actions.ChangePassword(config.DefaultPaths(), currentPassword, newPassword)
		if err != nil {
			return err
		}

		fmt.Println("Password changed successfully.")
		if detail := result.Detail; detail != "" {
			fmt.Println(detail)
		}
		return nil
	},
}
