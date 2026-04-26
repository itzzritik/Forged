package cmd

import (
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var (
	sshRouteAttempt      string
	sshRouteHost         string
	sshRouteOriginalHost string
	sshRouteUser         string
	sshRoutePort         string
)

var sshRoutePrepareCmd = &cobra.Command{
	Use:           "__ssh-route-prepare",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("FORGED_SSH_ROUTE_SKIP") == "1" {
			os.Exit(1)
		}
		cwd, err := os.Getwd()
		if err != nil {
			debugSSHRoute("prepare cwd: %v", err)
			return nil
		}

		_, err = ctlClient().Call(ipc.CmdSSHRoutePrepare, ipc.SSHRoutePrepareArgs{
			Attempt:      sshRouteAttempt,
			ClientPID:    os.Getppid(),
			CWD:          cwd,
			Host:         sshRouteHost,
			OriginalHost: sshRouteOriginalHost,
			User:         sshRouteUser,
			Port:         sshRoutePort,
		})
		if err != nil {
			debugSSHRoute("prepare: %v", err)
		}
		return nil
	},
}

var sshRouteSuccessCmd = &cobra.Command{
	Use:           "__ssh-route-success",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call(ipc.CmdSSHRouteSuccess, ipc.SSHRouteSuccessArgs{
			Attempt:   sshRouteAttempt,
			ClientPID: os.Getppid(),
		})
		if err != nil {
			debugSSHRoute("success: %v", err)
		}
		return nil
	},
}

func init() {
	for _, routeCmd := range []*cobra.Command{sshRoutePrepareCmd, sshRouteSuccessCmd} {
		routeCmd.Flags().StringVar(&sshRouteAttempt, "attempt", "", "routing attempt token")
		routeCmd.Flags().StringVar(&sshRouteHost, "host", "", "effective host")
		routeCmd.Flags().StringVar(&sshRouteOriginalHost, "original-host", "", "original host")
		routeCmd.Flags().StringVar(&sshRouteUser, "user", "", "target user")
		routeCmd.Flags().StringVar(&sshRoutePort, "port", "22", "target port")
	}
}

func debugSSHRoute(format string, args ...any) {
	if os.Getenv("FORGED_SSH_ROUTE_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "forged ssh-route: "+format+"\n", args...)
}
