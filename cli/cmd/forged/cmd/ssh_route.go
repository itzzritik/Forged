package cmd

import (
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	"github.com/spf13/cobra"
)

var (
	sshRouteProvider string
	sshRouteKeyID    string
)

var sshRouteMatchCmd = &cobra.Command{
	Use:          "__ssh-route-match",
	Short:        "Internal SSH routing helper",
	Hidden:       true,
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		if sshRouteProvider == "" || sshRouteKeyID == "" {
			os.Exit(1)
		}
		paths := config.DefaultPaths()
		runtime := sshrouting.MatchRuntime{
			StatePath:      paths.SSHRoutingStateFile(),
			ManagedKeysDir: paths.SSHManagedKeysDir(),
			AgentSocket:    paths.AgentSocket(),
		}
		if sshrouting.MatchProviderKey(cwd, sshRouteProvider, sshRouteKeyID, runtime) {
			os.Exit(0)
		}
		os.Exit(1)
	},
}

func init() {
	sshRouteMatchCmd.Flags().StringVar(&sshRouteProvider, "provider", "", "provider name")
	sshRouteMatchCmd.Flags().StringVar(&sshRouteKeyID, "key-id", "", "provider key id")
}
