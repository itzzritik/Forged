package cmd

import (
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !jsonOutput && isInteractiveTerminal() {
				return runInteractiveIntent(tui.ResolveCommand([]string{"key"}, args))
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
