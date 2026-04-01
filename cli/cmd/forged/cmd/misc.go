package cmd

import (
	"fmt"

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
	RunE:  notImplemented("logs"),
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Test Argon2id speed and recommend parameters",
	RunE:  notImplemented("benchmark"),
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "First-time setup wizard",
	RunE:  notImplemented("setup"),
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
