package cmd

import "github.com/spf13/cobra"

func shouldLaunchKeyManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runKeyManager(cmd *cobra.Command) error {
	items := []managerItem{
		{
			Label: "Generate a new key",
			Run: func() error {
				return generateCmd.RunE(generateCmd, nil)
			},
		},
		{
			Label: "Import keys",
			Run: func() error {
				return importCmd.RunE(importCmd, nil)
			},
		},
		{
			Label: "List keys",
			Run: func() error {
				return listCmd.RunE(listCmd, nil)
			},
		},
	}

	return runManagerProgram("Keys", items)
}
