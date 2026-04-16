package cmd

import (
	"os"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var (
	sshRouteAttempt string
	sshRouteHost    string
	sshRouteUser    string
	sshRoutePort    string
)

var sshRoutePrepareCmd = &cobra.Command{
	Use:           "__ssh-route-prepare",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		_, err = ctlClient().Call(ipc.CmdSSHRoutePrepare, ipc.SSHRoutePrepareArgs{
			Attempt:   sshRouteAttempt,
			ClientPID: os.Getppid(),
			CWD:       cwd,
			Host:      sshRouteHost,
			User:      sshRouteUser,
			Port:      sshRoutePort,
		})
		return err
	},
}

var sshRouteSuccessCmd = &cobra.Command{
	Use:           "__ssh-route-success",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call(ipc.CmdSSHRouteSuccess, ipc.SSHRouteSuccessArgs{
			Attempt: sshRouteAttempt,
		})
		return err
	},
}

func init() {
	for _, routeCmd := range []*cobra.Command{sshRoutePrepareCmd, sshRouteSuccessCmd} {
		routeCmd.Flags().StringVar(&sshRouteAttempt, "attempt", "", "routing attempt token")
		routeCmd.Flags().StringVar(&sshRouteHost, "host", "", "effective host")
		routeCmd.Flags().StringVar(&sshRouteUser, "user", "", "target user")
		routeCmd.Flags().StringVar(&sshRoutePort, "port", "22", "target port")
	}
}
