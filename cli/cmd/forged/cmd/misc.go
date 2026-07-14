package cmd

import "github.com/spf13/cobra"

var signCmd = &cobra.Command{
	Use:    "sign",
	Short:  "Git signing helper (called by git)",
	Hidden: true,
	RunE:   notImplemented("sign"),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printVersion(cmd)
	},
}
