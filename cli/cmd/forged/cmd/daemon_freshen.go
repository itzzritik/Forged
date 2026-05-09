package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonFreshenQuiet bool

var daemonFreshenCmd = &cobra.Command{
	Use:   "__daemon-freshen",
	Short: "Refresh an installed daemon when it is running an older build",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		runtime, err := daemon.DefaultRuntimeSpec()
		if err != nil {
			return err
		}
		updated, err := daemon.RefreshInstalledServiceIfStale(paths, runtime)
		if err != nil {
			return err
		}
		if daemonFreshenQuiet {
			return nil
		}
		if updated {
			fmt.Fprintln(cmd.OutOrStdout(), "Forged daemon refreshed")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Forged daemon already fresh")
		return nil
	},
}

func init() {
	daemonFreshenCmd.Flags().BoolVar(&daemonFreshenQuiet, "quiet", false, "suppress daemon freshness output")
}
