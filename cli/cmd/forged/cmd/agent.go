package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable Forged as the system SSH agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if err := config.EnableSSHAgent(paths); err != nil {
			return fmt.Errorf("enabling SSH agent: %w", err)
		}

		fmt.Println("Forged SSH agent enabled")
		fmt.Printf("  Forged SSH include configured at %s\n", paths.SSHBaseInclude())
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable Forged as the system SSH agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if err := config.DisableSSHAgent(paths); err != nil {
			return fmt.Errorf("disabling SSH agent: %w", err)
		}

		fmt.Println("Forged SSH agent disabled")
		fmt.Println("  Removed Forged include from ~/.ssh/config")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
}
