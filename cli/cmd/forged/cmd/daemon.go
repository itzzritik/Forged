package cmd

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/forgedkeys/forged/cli/internal/config"
	"github.com/forgedkeys/forged/cli/internal/daemon"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start daemon in foreground",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		password, err := readPassword("Master password: ")
		if err != nil {
			return err
		}

		d := daemon.New(paths)
		return d.Run(password)
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start daemon via system service",
	RunE:  notImplemented("start"),
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		pid, running := daemon.IsRunning(paths)
		if !running {
			return fmt.Errorf("daemon is not running")
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("finding process: %w", err)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("sending signal: %w", err)
		}

		fmt.Printf("Sent stop signal to daemon (PID %d)\n", pid)
		return nil
	},
}

func readPassword(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, prompt)
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

func readPID(paths config.Paths) (int, error) {
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}
