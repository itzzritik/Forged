package cmd

import "github.com/spf13/cobra"

func shouldLaunchVaultManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runVaultManager(cmd *cobra.Command) error {
	items := []managerItem{
		{
			Label: "Lock vault",
			Run: func() error {
				return lockCmd.RunE(lockCmd, nil)
			},
		},
		{
			Label: "Change password",
			Run: func() error {
				return changePasswordCmd.RunE(changePasswordCmd, nil)
			},
		},
	}

	return runManagerProgram("Vault", items)
}
