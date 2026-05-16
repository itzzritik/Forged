package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and repair this device",
	Long: "Open Forged on the Doctor tab. With --fix, run a headless repair pass first " +
		"so a broken install can be recovered before the TUI tries to start.",
	RunE: runDoctorCommand,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Run repairs before opening the Doctor tab")
}

func runDoctorCommand(cmd *cobra.Command, args []string) error {
	if doctorFix {
		if err := runHeadlessDoctorFix(); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "Pre-fix encountered an error:", err)
		}
	}
	if !isInteractiveTerminal() {
		return fmt.Errorf("Forged requires an interactive terminal. Re-run from a TTY")
	}
	return runInteractiveIntent(tui.DoctorIntent())
}

func runHeadlessDoctorFix() error {
	paths := config.DefaultPaths()
	engine := readiness.New(paths)
	_, err := engine.Run(readiness.RunOptions{Mode: readiness.ModeNonInteractiveFix})
	return err
}
