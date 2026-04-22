package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/spf13/cobra"
)

var signCmd = &cobra.Command{
	Use:    "sign",
	Short:  "Git signing helper (called by git)",
	Hidden: true,
	RunE:   notImplemented("sign"),
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail daemon logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		logPath := paths.LogFile()
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return fmt.Errorf("No log file found at %s", logPath)
		}
		tailCmd := exec.Command("tail", "-f", logPath)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr
		return tailCmd.Run()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printVersion(cmd)
	},
}
