package cmd

import "github.com/spf13/cobra"

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldLaunchKeyManager(args) {
				return runKeyManager(cmd)
			}
			return cmd.Help()
		},
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(
		importCmd,
		exportCmd,
		generateCmd,
		listCmd,
		viewCmd,
		renameCmd,
		removeCmd,
	)
	cmd.InitDefaultHelpCmd()
	configureGroupHelp(cmd, "Manage keys", []string{
		"forged key",
		"forged key generate",
		"forged key import",
		"forged key export",
		"forged key view github",
	})

	return cmd
}
