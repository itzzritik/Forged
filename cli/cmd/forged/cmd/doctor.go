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
	Short: "Diagnose and fix common issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		fix, _ := cmd.Flags().GetBool("fix")
		issues := 0

		fmt.Println("Forged Doctor")
		fmt.Println()

		if _, err := os.Stat(paths.VaultFile()); err == nil {
			pass("Vault exists", paths.VaultFile())
		} else {
			fail("Vault not found", "Run: forged setup")
			issues++
		}

		if _, err := os.Stat(paths.ConfigFile()); err == nil {
			pass("Config exists", paths.ConfigFile())
		} else {
			warn("Config not found", "Run: forged setup")
		}

		if pid, running := daemon.IsRunning(paths); running {
			pass("Daemon running", fmt.Sprintf("PID %d", pid))
		} else {
			fail("Daemon not running", "Run: forged start")
			issues++
		}

		if conn, err := net.DialTimeout("unix", paths.AgentSocket(), time.Second); err == nil {
			conn.Close()
			pass("Agent socket", paths.AgentSocket())
		} else {
			fail("Agent socket not responding", paths.AgentSocket())
			issues++
		}

		if conn, err := net.DialTimeout("unix", paths.CtlSocket(), time.Second); err == nil {
			conn.Close()
			pass("IPC socket", paths.CtlSocket())
		} else {
			fail("IPC socket not responding", paths.CtlSocket())
			issues++
		}

		if config.IsSSHAgentEnabled(paths) {
			pass("SSH agent", "Forged managed SSH files are configured")
		} else {
			if fix {
				if err := config.EnableSSHAgent(paths); err == nil {
					fixed("SSH agent", "Forged SSH include configured")
				} else {
					fail("SSH agent", fmt.Sprintf("could not fix: %v", err))
					issues++
				}
			} else {
				fail("SSH agent", "Forged SSH include not configured. Run: forged doctor --fix")
				issues++
			}
		}

		if daemon.ServiceInstalled() {
			pass("System service", "installed")
		} else {
			warn("System service", "not installed (daemon won't auto-start)")
		}

		if _, err := os.Stat(credentialsPath()); err == nil {
			pass("Cloud credentials", "logged in")
		} else {
			warn("Cloud credentials", "not logged in (sync disabled)")
		}

		fmt.Println()
		if issues == 0 {
			fmt.Println("Everything looks good.")
		} else if fix {
			fmt.Println("Some issues were fixed. Run forged doctor again to verify.")
		} else {
			fmt.Printf("%d issue(s) found. Run: forged doctor --fix\n", issues)
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "auto-fix issues where possible")
	rootCmd.AddCommand(doctorCmd)
}

func pass(label, detail string) {
	fmt.Printf("  [PASS]  %-25s %s\n", label, detail)
}

func fail(label, detail string) {
	fmt.Printf("  [FAIL]  %-25s %s\n", label, detail)
}

func warn(label, detail string) {
	fmt.Printf("  [WARN]  %-25s %s\n", label, detail)
}

func fixed(label, detail string) {
	fmt.Printf("  [FIXED] %-25s %s\n", label, detail)
}
