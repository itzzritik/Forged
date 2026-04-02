package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

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

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start daemon as background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if _, running := daemon.IsRunning(paths); running {
			fmt.Println("Daemon is already running")
			return nil
		}

		if !daemon.ServiceInstalled() {
			return fmt.Errorf("service not installed. Run: forged setup")
		}

		if err := daemon.StartService(); err != nil {
			return fmt.Errorf("starting service: %w", err)
		}

		time.Sleep(2 * time.Second)

		if pid, running := daemon.IsRunning(paths); running {
			fmt.Printf("Daemon started (PID %d)\n", pid)
		} else {
			fmt.Println("Service started. Check: forged logs")
		}
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()

		if daemon.ServiceInstalled() {
			if err := daemon.StopService(); err != nil {
				return fmt.Errorf("stopping service: %w", err)
			}
			fmt.Println("Daemon stopped")
			return nil
		}

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

		fmt.Printf("Daemon stopped (PID %d)\n", pid)
		return nil
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
