package cmd

import (
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/launcher"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/spf13/cobra"
)

var isInteractiveTerminal = terminalIsInteractive

func shouldLaunchBareForged(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runBareForged(cmd *cobra.Command) error {
	paths := config.DefaultPaths()
	controller := launcher.NewController(launcher.Dependencies{
		Readiness: readiness.New(paths),
		Actions: map[launcher.ActionID]launcher.ActionFunc{
			launcher.ActionSetupVault: func(snapshot readiness.Snapshot) (string, error) {
				return runSetupVault(paths)
			},
			launcher.ActionGenerate: func(snapshot readiness.Snapshot) (string, error) {
				if err := generateCmd.RunE(generateCmd, nil); err != nil {
					return "", err
				}
				return "", nil
			},
			launcher.ActionImport: func(snapshot readiness.Snapshot) (string, error) {
				if err := importCmd.RunE(importCmd, nil); err != nil {
					return "", err
				}
				return "", nil
			},
			launcher.ActionSigning: func(snapshot readiness.Snapshot) (string, error) {
				if err := signingCmd.RunE(signingCmd, nil); err != nil {
					return "", err
				}
				return "", nil
			},
			launcher.ActionLogin: func(snapshot readiness.Snapshot) (string, error) {
				if err := loginCmd.RunE(loginCmd, nil); err != nil {
					return "", err
				}
				return "", nil
			},
		},
	})
	return controller.Run()
}

func runSetupVault(paths config.Paths) (string, error) {
	if _, err := os.Stat(paths.VaultFile()); err == nil {
		return "", fmt.Errorf("vault already exists at %s", paths.VaultFile())
	}

	password, err := createPassword()
	if err != nil {
		return "", err
	}

	v, _, err := createVaultAtPaths(paths, password)
	if err != nil {
		return "", err
	}
	defer v.Close()

	if err := ensureDefaultConfig(paths); err != nil {
		return "", err
	}
	if err := config.EnableSSHAgent(paths); err != nil {
		return "", err
	}
	if err := ensureLocalService(paths, password); err != nil {
		return "", err
	}

	return "New vault created", nil
}
