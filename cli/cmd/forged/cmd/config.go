package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	RunE:  notImplemented("config"),
}

func init() {
	configCmd.AddCommand(
		&cobra.Command{
			Use:   "get <key>",
			Short: "Get config value",
			Args:  cobra.ExactArgs(1),
			RunE:  notImplemented("config get"),
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set config value",
			Args:  cobra.ExactArgs(2),
			RunE:  notImplemented("config set"),
		},
	)
}
