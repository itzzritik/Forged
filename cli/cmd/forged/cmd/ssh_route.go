package cmd

import (
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	"github.com/spf13/cobra"
)

var sshRouteKeyID string

var sshRouteMatchCmd = &cobra.Command{
	Use:                "__ssh-route-match",
	Hidden:             true,
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		if sshRouteKeyID == "" {
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

		if keyID, ok := state.RepoRoutes[remote.RouteKey]; ok {
			if keyID == sshRouteKeyID {
				os.Exit(0)
			}
			os.Exit(1)
		}

		account, ok := state.GitHubAccounts[sshRouteKeyID]
		if !ok || account == "" {
			account, err = sshrouting.ProbeGitHubAccount(
				paths.AgentSocket(),
				sshrouting.HintFilePath(paths.SSHManagedKeysDir(), sshRouteKeyID),
			)
			if err != nil {
				os.Exit(1)
			}
			state.GitHubAccounts[sshRouteKeyID] = account
			if err := sshrouting.SaveState(paths.SSHRoutingStateFile(), state); err != nil {
				os.Exit(1)
			}
		}

		if account != remote.Owner {
			os.Exit(1)
		}

		state.RepoRoutes[remote.RouteKey] = sshRouteKeyID
		if err := sshrouting.SaveState(paths.SSHRoutingStateFile(), state); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	},
}

func init() {
	sshRouteMatchCmd.Flags().StringVar(&sshRouteKeyID, "key-id", "", "internal key route matcher")
	rootCmd.AddCommand(sshRouteMatchCmd)
}
