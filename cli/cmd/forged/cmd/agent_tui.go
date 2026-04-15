package cmd

import "github.com/spf13/cobra"

func shouldLaunchAgentManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runAgentManager(cmd *cobra.Command) error {
	items := []managerItem{
		{
			Label: "Use Forged as your SSH agent",
			Run: func() error {
				return enableCmd.RunE(enableCmd, nil)
			},
		},
		{
			Label: "Stop using Forged as your SSH agent",
			Run: func() error {
				return disableCmd.RunE(disableCmd, nil)
			},
		},
		{
			Label: "Use Forged for Git signing",
			Run: func() error {
				return signingCmd.RunE(signingCmd, nil)
			},
		},
	}

	return runManagerProgram("Agent", items)
}
