package cmd

import "github.com/spf13/cobra"

func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vault access",
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldLaunchVaultManager(args) {
				return runVaultManager(cmd)
			}
			return cmd.Help()
		},
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(lockCmd, unlockCmd, changePasswordCmd)
	cmd.InitDefaultHelpCmd()
	configureGroupHelp(cmd, "Manage vault access", []string{
		"forged vault",
		"forged vault unlock",
		"forged vault change-password",
	})

	return cmd
}
