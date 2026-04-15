package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	jsonOutput    bool
	verbose       bool
	configPath    string
	versionOutput bool

	version = "dev"
	commit  = "none"
)

var retiredCommandHints = map[string]string{
	"generate": "Run `forged key generate`.",
	"list":     "Run `forged key list`.",
	"view":     "Run `forged key view <name>`.",
	"rename":   "Run `forged key rename <old> <new>`.",
	"remove":   "Run `forged key delete <name>`.",
	"import":   "Run `forged key import`.",
	"export":   "Run `forged key export`.",
	"setup":    "Run `forged`.",
	"start":    "Run `forged` or `forged doctor --fix`.",
	"stop":     "Forged now manages the service automatically.",
	"status":   "Run `forged`.",
	"daemon":   "Forged now manages the service automatically.",
	"config":   "Run `forged` or `forged doctor`.",
	"register": "Run `forged login`.",
	"add":      "Run `forged key import`.",
	"version":  "Run `forged --version`.",
}

func Execute() error {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		err = rewriteCLIError(err)
		fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "forged",
		Short:         "Manage SSH keys, vault access, and signing with Forged",
		Long:          "Forged is a cross-platform SSH key manager with zero-knowledge encrypted sync and Git commit signing.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runRootCommand,
	}

	installPersistentFlags(cmd)
	installRootSubcommands(cmd)
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultCompletionCmd()
	for _, child := range cmd.Commands() {
		if child.Name() == "completion" {
			child.Hidden = true
		}
	}
	configureHelp(cmd)

	return cmd
}

func installPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose logging")
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "override config file path")
	cmd.Flags().BoolVarP(&versionOutput, "version", "v", false, "print version information")
}

func installRootSubcommands(cmd *cobra.Command) {
	cmd.CompletionOptions.HiddenDefaultCmd = true

	logsCmd.Hidden = true
	signCmd.Hidden = true
	sshRouteMatchCmd.Hidden = true

	cmd.AddCommand(
		loginCmd,
		logoutCmd,
		syncCmd,
		doctorCmd,
		newKeyCmd(),
		newVaultCmd(),
		newAgentCmd(),
		logsCmd,
		signCmd,
		sshRouteMatchCmd,
	)
}

func runRootCommand(cmd *cobra.Command, args []string) error {
	if versionOutput {
		return printVersion(cmd)
	}
	if shouldLaunchBareForged(args) {
		return runBareForged(cmd)
	}
	return runRootSummary()
}

func rewriteCLIError(err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	for removed, hint := range retiredCommandHints {
		target := fmt.Sprintf("unknown command %q for %q", removed, "forged")
		if strings.Contains(message, target) {
			return fmt.Errorf("%s %s", message, hint)
		}
	}

	return err
}

func printVersion(cmd *cobra.Command) error {
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]string{
			"version": version,
			"commit":  commit,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "forged %s (%s)\n", version, commit)
	return nil
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
