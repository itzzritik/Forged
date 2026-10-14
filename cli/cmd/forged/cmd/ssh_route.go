package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
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
	sshRouteSocket       string
)

var sshRoutePrepareCmd = &cobra.Command{
	Use:           "__ssh-route-prepare",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !platform.SSHRoutingPlatform() {
			return fmt.Errorf("SSH routing is unavailable on this platform")
		}
		if os.Getenv("FORGED_SSH_ROUTE_SKIP") == "1" {
			os.Exit(1)
		}
		cwd, err := os.Getwd()
		if err != nil {
			debugSSHRoute("prepare cwd: %v", err)
			return nil
		}

		resp, err := routeClient().CallWithTimeout(ipc.CmdSSHRoutePrepare, ipc.SSHRoutePrepareArgs{
			Attempt:      sshRouteAttempt,
			ClientPID:    os.Getppid(),
			CWD:          cwd,
			Host:         sshRouteHost,
			OriginalHost: sshRouteOriginalHost,
			User:         sshRouteUser,
			Port:         sshRoutePort,
		}, ipc.SSHRoutePrepareCallTimeout)
		var result ipc.SSHRoutePrepareResult
		if json.Unmarshal(resp.Data, &result) == nil && result.Locked {
			fmt.Fprintln(os.Stderr, "forged: locked. Run `forged` to unlock.")
		} else if err != nil {
			// Without a prepared route ssh offers no Forged key; say why instead of a bare "Permission denied".
			fmt.Fprintf(os.Stderr, "forged: SSH keys unavailable: %s\n", ui.Sanitize(err.Error()))
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
		if !platform.SSHRoutingPlatform() {
			return fmt.Errorf("SSH routing is unavailable on this platform")
		}
		_, err := routeClient().CallWithTimeout(ipc.CmdSSHRouteSuccess, ipc.SSHRouteSuccessArgs{
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
		if !platform.SSHRoutingPlatform() {
			os.Exit(1)
		}
		_, err := routeClient().CallWithTimeout(ipc.CmdSSHRouteSlot, ipc.SSHRouteSlotArgs{
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
		routeCmd.Flags().StringVar(&sshRouteSocket, "socket", "", "daemon control socket")
	}
	for _, routeCmd := range []*cobra.Command{sshRoutePrepareCmd, sshRouteSuccessCmd} {
		routeCmd.Flags().StringVar(&sshRouteHost, "host", "", "effective host")
		routeCmd.Flags().StringVar(&sshRouteOriginalHost, "original-host", "", "original host")
		routeCmd.Flags().StringVar(&sshRouteUser, "user", "", "target user")
		routeCmd.Flags().StringVar(&sshRoutePort, "port", "22", "target port")
	}
	sshRouteSlotCmd.Flags().IntVar(&sshRouteSlot, "slot", 0, "routing identity slot")
}

// The daemon passes its socket in the managed config: ssh reads that config from the real home,
// but a caller may run it with another $HOME (tests, CI), where the default path finds no daemon.
func routeClient() *ipc.Client {
	if sshRouteSocket != "" {
		return ipc.NewClient(sshRouteSocket)
	}
	return ctlClient()
}

func debugSSHRoute(format string, args ...any) {
	if os.Getenv("FORGED_SSH_ROUTE_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "forged ssh-route: "+format+"\n", args...)
}
