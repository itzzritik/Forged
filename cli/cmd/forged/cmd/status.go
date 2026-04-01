package cmd

import (
	"fmt"

	"github.com/forgedkeys/forged/cli/internal/config"
	"github.com/forgedkeys/forged/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status, key count, sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		pid, running := daemon.IsRunning(paths)

		if jsonOutput {
			return printOutput(map[string]any{
				"running": running,
				"pid":     pid,
				"socket":  paths.AgentSocket(),
			})
		}

		if !running {
			fmt.Println("Daemon: not running")
			fmt.Printf("Socket: %s\n", paths.AgentSocket())
			return nil
		}

		fmt.Printf("Daemon: running (PID %d)\n", pid)
		fmt.Printf("Socket: %s\n", paths.AgentSocket())
		return nil
	},
}

func statusRun(cmd *cobra.Command, args []string) error {
	return statusCmd.RunE(cmd, args)
}
