package cmd

import (
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

func shouldLaunchVaultManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runVaultManager(cmd *cobra.Command) error {
	return runInteractiveIntent(tui.ResolveCommand([]string{"vault"}, nil))
}
