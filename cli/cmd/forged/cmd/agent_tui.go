package cmd

import (
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

func shouldLaunchAgentManager(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runAgentManager(cmd *cobra.Command) error {
	return runInteractiveIntent(tui.ResolveCommand([]string{"agent"}, nil))
}
