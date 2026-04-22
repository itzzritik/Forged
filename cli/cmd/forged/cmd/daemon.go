package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start daemon in foreground",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		password, err := getStartupPassword()
		if err != nil {
			return err
		}

		d := daemon.New(paths)
		return d.Run(password)
	},
}

func getStartupPassword() ([]byte, error) {
	if env := os.Getenv("FORGED_MASTER_PASSWORD"); env != "" {
		return []byte(env), nil
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		return nil, nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspecting stdin: %w", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 && !info.Mode().IsRegular() {
		return nil, nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading password from stdin: %w", err)
	}
	password := data
	if len(password) > 0 && password[len(password)-1] == '\n' {
		password = password[:len(password)-1]
	}
	return password, nil
}
