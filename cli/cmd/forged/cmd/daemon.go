package cmd

import (
	"fmt"
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

		password, err := getPassword()
		if err != nil {
			return err
		}

		d := daemon.New(paths)
		return d.Run(password)
	},
}

func getPassword() ([]byte, error) {
	if env := os.Getenv("FORGED_MASTER_PASSWORD"); env != "" {
		return []byte(env), nil
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Master password: ")
		password, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("reading password: %w", err)
		}
		return password, nil
	}

	var buf [1024]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return nil, fmt.Errorf("reading password from stdin: %w", err)
	}

	password := buf[:n]
	if len(password) > 0 && password[len(password)-1] == '\n' {
		password = password[:len(password)-1]
	}
	return password, nil
}
