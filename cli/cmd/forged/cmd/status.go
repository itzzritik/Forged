package cmd

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status, key count, sync status",
	RunE:  notImplemented("status"),
}

func statusRun(cmd *cobra.Command, args []string) error {
	return statusCmd.RunE(cmd, args)
}
