package cmd

import "github.com/spf13/cobra"

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Use Forged for SSH and Git signing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if shouldLaunchAgentManager(args) {
				return runAgentManager(cmd)
			}
			return cmd.Help()
		},
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.AddCommand(enableCmd, disableCmd, signingCmd)
	cmd.InitDefaultHelpCmd()
	configureGroupHelp(cmd, "Use Forged for SSH and Git signing", []string{
		"forged agent",
		"forged agent enable",
		"forged agent signing",
	})

	return cmd
}
