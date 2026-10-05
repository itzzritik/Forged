package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const maxStartupPasswordBytes = 64 * 1024

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start daemon in foreground",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		password, err := getStartupPassword()
		if err != nil {
			return err
		}
		platform.DetachOwnedConsole()

		d := daemon.New(paths)
		return d.Run(password)
	},
}

func getStartupPassword() ([]byte, error) {
	if _, set := os.LookupEnv("FORGED_MASTER_PASSWORD"); set {
		return nil, fmt.Errorf("FORGED_MASTER_PASSWORD is unsupported; remove it and run forged doctor --fix")
	}

	// mintty without ConPTY connects stdin to a pipe, which would otherwise be
	// read as a startup password and block until the terminal closes.
	if term.IsTerminal(int(os.Stdin.Fd())) || platform.IsMSYSTerminal(os.Stdin) {
		return nil, nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, nil
	}
	if info.Mode()&os.ModeNamedPipe == 0 && !info.Mode().IsRegular() {
		return nil, nil
	}

	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxStartupPasswordBytes+1))
	if err != nil {
		clearStartupPassword(data)
		return nil, fmt.Errorf("Reading password from stdin: %w", err)
	}
	if len(data) > maxStartupPasswordBytes {
		clearStartupPassword(data)
		return nil, fmt.Errorf("Startup password is too long")
	}
	password := data
	if len(password) > 0 && password[len(password)-1] == '\n' {
		password[len(password)-1] = 0
		password = password[:len(password)-1]
		if len(password) > 0 && password[len(password)-1] == '\r' {
			password[len(password)-1] = 0
			password = password[:len(password)-1]
		}
	}
	return password, nil
}

func clearStartupPassword(password []byte) {
	for i := range password {
		password[i] = 0
	}
}
