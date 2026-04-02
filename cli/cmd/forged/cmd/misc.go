package cmd

import (
	"fmt"
	"os"
	"syscall"

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
			return fmt.Errorf("no log file found at %s", logPath)
		}
		return syscall.Exec("/usr/bin/tail", []string{"tail", "-f", logPath}, os.Environ())
	},
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Test Argon2id speed and recommend parameters",
	RunE:  notImplemented("benchmark"),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOutput {
			return printOutput(map[string]string{
				"version": version,
				"commit":  commit,
			})
		}
		fmt.Printf("forged %s (%s)\n", version, commit)
		return nil
	},
}
