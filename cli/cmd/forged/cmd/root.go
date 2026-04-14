package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	verbose    bool
	configPath string

	version = "dev"
	commit  = "none"
)

var rootCmd = &cobra.Command{
	Use:   "forged",
	Short: "SSH key management — forge your keys, take them anywhere",
	Long:  "Forged is a cross-platform SSH key manager with zero-knowledge encrypted sync, intelligent host matching, and Git commit signing.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusRun(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose logging")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "override config file path")

	rootCmd.AddCommand(
		daemonCmd,
		startCmd,
		stopCmd,
		statusCmd,
		addCmd,
		generateCmd,
		listCmd,
		removeCmd,
		exportCmd,
		viewCmd,
		renameCmd,
		hostCmd,
		hostsCmd,
		unhostCmd,
		syncCmd,
		signCmd,
		loginCmd,
		logoutCmd,
		registerCmd,
		sshRouteMatchCmd,
		lockCmd,
		unlockCmd,
		configCmd,
		logsCmd,
		benchmarkCmd,
		setupCmd,
		versionCmd,
		changePasswordCmd,
	)
}

func printOutput(data any) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	fmt.Println(data)
	return nil
}

func notImplemented(name string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: not yet implemented", name)
	}
}
