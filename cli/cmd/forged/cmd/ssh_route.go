package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/sshrouting"
	"github.com/spf13/cobra"
)

const sshRouteSuccessCallTimeout = 5 * time.Second

var (
	sshRouteAttempt      string
	sshRouteHost         string
	sshRouteOriginalHost string
	sshRouteUser         string
	sshRoutePort         string
	sshRouteSlot         int
)

var sshRoutePrepareCmd = &cobra.Command{
	Use:           "__ssh-route-prepare",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.SSHRoutingSupported() {
			return fmt.Errorf("SSH routing is unavailable on this platform")
		}
		if os.Getenv("FORGED_SSH_ROUTE_SKIP") == "1" {
			os.Exit(1)
		}
		if err := sshrouting.RemoveRouteReady(config.DefaultPaths().SSHRouteRuntimeDir(), sshRouteAttempt); err != nil {
			debugSSHRoute("prepare ready marker: %v", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			debugSSHRoute("prepare cwd: %v", err)
			return nil
		}

		_, err = ctlClient().CallWithTimeout(ipc.CmdSSHRoutePrepare, ipc.SSHRoutePrepareArgs{
			Attempt:      sshRouteAttempt,
			ClientPID:    os.Getppid(),
			CWD:          cwd,
			Host:         sshRouteHost,
			OriginalHost: sshRouteOriginalHost,
			User:         sshRouteUser,
			Port:         sshRoutePort,
		}, ipc.SSHRoutePrepareCallTimeout)
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
	Args:          cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.SSHRoutingSupported() {
			return fmt.Errorf("SSH routing is unavailable on this platform")
		}
		_, err := ctlClient().CallWithTimeout(ipc.CmdSSHRouteSuccess, ipc.SSHRouteSuccessArgs{
			Attempt:   sshRouteAttempt,
			ClientPID: os.Getppid(),
		}, sshRouteSuccessCallTimeout)
		if err != nil {
			debugSSHRoute("success: %v", err)
		}
		return nil
	},
}

var sshRouteSlotCmd = &cobra.Command{
	Use:           "__ssh-route-slot",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !platform.SSHRoutingSupported() {
			os.Exit(1)
		}
		_, err := ctlClient().CallWithTimeout(ipc.CmdSSHRouteSlot, ipc.SSHRouteSlotArgs{
			Attempt:   sshRouteAttempt,
			ClientPID: os.Getppid(),
			Slot:      sshRouteSlot,
		}, sshRouteSuccessCallTimeout)
		if err != nil {
			debugSSHRoute("slot: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	for _, routeCmd := range []*cobra.Command{sshRoutePrepareCmd, sshRouteSuccessCmd, sshRouteSlotCmd} {
		routeCmd.Flags().StringVar(&sshRouteAttempt, "attempt", "", "routing attempt token")
	}
	for _, routeCmd := range []*cobra.Command{sshRoutePrepareCmd, sshRouteSuccessCmd} {
		routeCmd.Flags().StringVar(&sshRouteHost, "host", "", "effective host")
		routeCmd.Flags().StringVar(&sshRouteOriginalHost, "original-host", "", "original host")
		routeCmd.Flags().StringVar(&sshRouteUser, "user", "", "target user")
		routeCmd.Flags().StringVar(&sshRoutePort, "port", "22", "target port")
	}
	sshRouteSlotCmd.Flags().IntVar(&sshRouteSlot, "slot", 0, "routing identity slot")
}

func debugSSHRoute(format string, args ...any) {
	if os.Getenv("FORGED_SSH_ROUTE_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "forged ssh-route: "+format+"\n", args...)
}
