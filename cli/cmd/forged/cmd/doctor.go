package cmd

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		ok := true

		fmt.Println("Forged Doctor")
		fmt.Println()

		// Vault
		if _, err := os.Stat(paths.VaultFile()); err == nil {
			pass("Vault exists", paths.VaultFile())
		} else {
			fail("Vault not found", "Run: forged setup")
			ok = false
		}

		// Config
		if _, err := os.Stat(paths.ConfigFile()); err == nil {
			pass("Config exists", paths.ConfigFile())
		} else {
			warn("Config not found", "Run: forged setup")
		}

		// Daemon
		if pid, running := daemon.IsRunning(paths); running {
			pass("Daemon running", fmt.Sprintf("PID %d", pid))
		} else {
			fail("Daemon not running", "Run: forged start")
			ok = false
		}

		// Agent socket
		if conn, err := net.DialTimeout("unix", paths.AgentSocket(), time.Second); err == nil {
			conn.Close()
			pass("Agent socket", paths.AgentSocket())
		} else {
			fail("Agent socket not responding", paths.AgentSocket())
			ok = false
		}

		// IPC socket
		if conn, err := net.DialTimeout("unix", paths.CtlSocket(), time.Second); err == nil {
			conn.Close()
			pass("IPC socket", paths.CtlSocket())
		} else {
			fail("IPC socket not responding", paths.CtlSocket())
			ok = false
		}

		// SSH config
		home, _ := os.UserHomeDir()
		sshConfig := home + "/.ssh/config"
		if data, err := os.ReadFile(sshConfig); err == nil {
			if contains(string(data), "forged") {
				pass("SSH config", "IdentityAgent configured")
			} else {
				warn("SSH config", "IdentityAgent not pointing to Forged")
			}
		} else {
			warn("SSH config", "~/.ssh/config not found")
		}

		// Service
		if daemon.ServiceInstalled() {
			pass("Launchd service", "installed")
		} else {
			warn("Launchd service", "not installed (daemon won't auto-start)")
		}

		// Credentials
		if _, err := os.Stat(credentialsPath()); err == nil {
			pass("Cloud credentials", "logged in")
		} else {
			warn("Cloud credentials", "not logged in (sync disabled)")
		}

		fmt.Println()
		if ok {
			fmt.Println("Everything looks good.")
		} else {
			fmt.Println("Issues found. Fix the items marked with [FAIL] above.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func pass(label, detail string) {
	fmt.Printf("  [PASS] %-25s %s\n", label, detail)
}

func fail(label, detail string) {
	fmt.Printf("  [FAIL] %-25s %s\n", label, detail)
}

func warn(label, detail string) {
	fmt.Printf("  [WARN] %-25s %s\n", label, detail)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
