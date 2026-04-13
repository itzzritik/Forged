package cmd

import (
	"fmt"
	"strings"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock sensitive CLI access",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call(ipc.CmdSensitiveLock, nil)
		if err != nil {
			if strings.Contains(err.Error(), "daemon is not running") {
				fmt.Println("Sensitive CLI access already locked.")
				return nil
			}
			return err
		}
		fmt.Println("Sensitive CLI access locked.")
		return nil
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock vault",
	RunE:  notImplemented("unlock"),
}
