package cmd

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/itzzritik/forged/cli/internal/buildinfo"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/update"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const updateHintEvery = 24 * time.Hour

// The npm launcher answers `forged update` itself; this only runs without it.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update forged to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		latest, err := update.Latest(ctx)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}
		if !update.Available(buildinfo.Version, latest) {
			fmt.Fprintf(cmd.OutOrStdout(), "forged is up to date (%s)\n", buildinfo.Version)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "forged %s is out. Update with: npm i -g forgedssh@latest\n", latest)
		return nil
	},
}

func printUpdateHint(cmd *cobra.Command) {
	if !cmd.HasParent() || cmd.Hidden || slices.Contains([]string{"update", "daemon", "doctor"}, cmd.Name()) || rootInUserHome() {
		return
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}
	paths := config.DefaultPaths()
	latest := update.Load(paths)
	if !update.Available(buildinfo.Version, latest) {
		return
	}
	if info, err := os.Stat(paths.UpdateHintFile()); err == nil && time.Since(info.ModTime()) < updateHintEvery {
		return
	}
	if os.WriteFile(paths.UpdateHintFile(), nil, 0o600) != nil {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "forged %s is out. Run: forged update\n", latest)
}
