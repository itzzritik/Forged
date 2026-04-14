package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable Forged as the active SSH agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if err := config.EnableSSHAgent(paths); err != nil {
			return fmt.Errorf("enabling SSH agent: %w", err)
		}

		fmt.Println("Forged SSH agent enabled")
		fmt.Println("  New SSH connections will use Forged.")
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable Forged and remove its managed SSH config",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if err := config.DisableSSHAgent(paths); err != nil {
			return fmt.Errorf("disabling SSH agent: %w", err)
		}

		fmt.Println("Forged SSH agent disabled")
		fmt.Println("  Removed Forged-managed SSH config.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
}
