package cmd

import (
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	"github.com/spf13/cobra"
)

var sshRouteKeyRef string

var sshRouteMatchCmd = &cobra.Command{
	Use:                "__ssh-match",
	Aliases:            []string{"__ssh-route-match"},
	Hidden:             true,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		if sshRouteKeyRef == "" {
			os.Exit(1)
		}

		paths := config.DefaultPaths()
		remote, err := sshrouting.CurrentRemote()
		if err != nil || remote.Host != "github.com" || remote.Owner == "" {
			os.Exit(1)
		}

		state, err := sshrouting.LoadState(paths.SSHRoutingStateFile())
		if err != nil {
			os.Exit(1)
		}

		if keyRef, ok := state.Routes[remote.RouteKey]; ok {
			if keyRef == sshRouteKeyRef {
				os.Exit(0)
			}
			os.Exit(1)
		}

		account, err := sshrouting.ProbeGitHubAccount(
			paths.AgentSocket(),
			sshrouting.HintFilePath(paths.SSHManagedKeysDir(), sshRouteKeyRef),
		)
		if err != nil {
			os.Exit(1)
		}

		if account != remote.Owner {
			os.Exit(1)
		}

		state.Routes[remote.RouteKey] = sshRouteKeyRef
		if err := sshrouting.SaveState(paths.SSHRoutingStateFile(), state); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	},
}

func init() {
	sshRouteMatchCmd.Flags().StringVar(&sshRouteKeyRef, "key", "", "internal key route matcher")
	sshRouteMatchCmd.Flags().StringVar(&sshRouteKeyRef, "key-id", "", "internal key route matcher")
}
