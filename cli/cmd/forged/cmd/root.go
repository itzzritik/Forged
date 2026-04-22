package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	versionOutput bool

	version = "dev"
	commit  = "none"
)

var retiredCommandHints = map[string]string{
	"agent":    "Run `forged` and use the Agent tab.",
	"doctor":   "Run `forged` and use the Doctor tab.",
	"key":      "Run `forged` and use the Key tab.",
	"login":    "Run `forged` and use Manage > Log In.",
	"logout":   "Run `forged` and use Manage > Log Out.",
	"register": "Run `forged` and use Manage > Log In.",
	"setup":    "Run `forged`.",
	"status":   "Run `forged`.",
	"sync":     "Run `forged` and use Manage > Sync Now.",
	"vault":    "Run `forged` and use Manage.",
	"config":   "Run `forged` or `forged version`.",
	"generate": "Run `forged` and use the Key tab.",
	"list":     "Run `forged` and use the Key tab.",
	"view":     "Run `forged` and use the Key tab.",
	"rename":   "Run `forged` and use the Key tab.",
	"remove":   "Run `forged` and use the Key tab.",
	"import":   "Run `forged` and use the Key tab.",
	"export":   "Run `forged` and use the Key tab.",
	"add":      "Run `forged` and use the Key tab.",
	"version":  "Run `forged version` or `forged --version`.",
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
	cmd.Flags().BoolVarP(&versionOutput, "version", "v", false, "print version information")
}

func installRootSubcommands(cmd *cobra.Command) {
	cmd.CompletionOptions.HiddenDefaultCmd = true

	for _, hiddenCmd := range []*cobra.Command{
		daemonCmd,
		logsCmd,
		signCmd,
		sshRoutePrepareCmd,
		sshRouteSuccessCmd,
	} {
		hiddenCmd.Hidden = true
	}

	cmd.AddCommand(
		daemonCmd,
		versionCmd,
		logsCmd,
		signCmd,
		sshRoutePrepareCmd,
		sshRouteSuccessCmd,
	)
}

func runRootCommand(cmd *cobra.Command, args []string) error {
	if versionOutput {
		return printVersion(cmd)
	}
	if !shouldLaunchBareForged(args) {
		return fmt.Errorf("Forged requires an interactive terminal. Use `forged help` or `forged version`")
	}
	return runBareForged(cmd)
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
	fmt.Fprintf(cmd.OutOrStdout(), "forged %s (%s)\n", version, commit)
	return nil
}

func notImplemented(name string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("%s: not yet implemented", name)
	}
}
