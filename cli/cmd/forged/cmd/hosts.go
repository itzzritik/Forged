package cmd

import "github.com/spf13/cobra"

var hostCmd = &cobra.Command{
	Use:   "host <key-name> <patterns...>",
	Short: "Map a key to host patterns",
	Args:  cobra.MinimumNArgs(2),
	RunE:  notImplemented("host"),
}

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "List all host-key mappings",
	RunE:  notImplemented("hosts"),
}

var unhostCmd = &cobra.Command{
	Use:   "unhost <key-name> <pattern>",
	Short: "Remove a host mapping",
	Args:  cobra.ExactArgs(2),
	RunE:  notImplemented("unhost"),
}
