package cmd

import "github.com/spf13/cobra"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start daemon in foreground",
	RunE:  notImplemented("daemon"),
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start daemon via system service",
	RunE:  notImplemented("start"),
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop daemon",
	RunE:  notImplemented("stop"),
}
