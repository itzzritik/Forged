package cmd

import (
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock private-key access",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !jsonOutput && isInteractiveTerminal() {
			return runInteractiveIntent(tui.ResolveCommand([]string{"vault", "lock"}, args))
		}
		_, err := ctlClient().Call(ipc.CmdSensitiveLock, nil)
		if err != nil {
			if strings.Contains(err.Error(), "daemon is not running") {
				fmt.Println("Private-key access already locked.")
				return nil
			}
			return err
		}
		fmt.Println("Private-key access locked.")
		return nil
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !jsonOutput && isInteractiveTerminal() {
			return runInteractiveIntent(tui.ResolveCommand([]string{"vault", "unlock"}, args))
		}
		return notImplemented("unlock")(cmd, args)
	},
}
