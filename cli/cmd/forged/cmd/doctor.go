package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and fix common issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		fix, _ := cmd.Flags().GetBool("fix")
		engine := readiness.New(paths)

		fmt.Println("Forged Doctor")
		fmt.Println()

		snapshot, err := engine.Assess()
		if err != nil {
			return err
		}

		var summary readiness.RepairSummary
		if fix {
			snapshot, summary, err = engine.Repair(snapshot)
			if err != nil {
				return err
			}
		}

		renderDoctorVault(snapshot, summary)
		renderDoctorConfig(snapshot, summary)
		renderDoctorRuntime(snapshot, summary)
		renderDoctorSSH(snapshot, summary, paths)
		renderDoctorIdentityOwner(snapshot)
		renderDoctorService(snapshot)
		renderDoctorLogin(snapshot)

		issues := doctorIssueCount(snapshot)

		fmt.Println()
		switch {
		case issues == 0 && len(summary.Fixed) == 0:
			fmt.Println("Everything looks good.")
		case issues == 0 && len(summary.Fixed) > 0:
			fmt.Println("All detected issues were fixed.")
		case issues > 0 && len(summary.Fixed) > 0:
			fmt.Printf("Some issues were fixed, but %d issue(s) remain.\n", issues)
		case fix:
			fmt.Println("Issues remain and no safe automatic fix was available for some items.")
		default:
			fmt.Printf("%d issue(s) found. Run: forged doctor --fix\n", issues)
		}
		return nil
	},
}

func renderDoctorVault(snapshot readiness.Snapshot, summary readiness.RepairSummary) {
	if snapshot.VaultExists {
		pass("Vault exists", config.DefaultPaths().VaultFile())
		return
	}
	fail("Vault not found", "Run: forged setup")
}

func renderDoctorConfig(snapshot readiness.Snapshot, summary readiness.RepairSummary) {
	switch {
	case snapshot.ConfigExists && containsFix(summary, "config"):
		fixed("Config exists", config.DefaultPaths().ConfigFile())
	case snapshot.ConfigExists:
		pass("Config exists", config.DefaultPaths().ConfigFile())
	default:
		fail("Config not found", "Run: forged setup")
	}
}

func renderDoctorRuntime(snapshot readiness.Snapshot, summary readiness.RepairSummary) {
	switch {
	case snapshot.Service.Running && containsFix(summary, "service") && snapshot.DaemonPID > 0:
		fixed("Daemon running", fmt.Sprintf("PID %d", snapshot.DaemonPID))
	case snapshot.Service.Running && snapshot.DaemonPID > 0:
		pass("Daemon running", fmt.Sprintf("PID %d", snapshot.DaemonPID))
	default:
		fail("Daemon not running", "Run: forged start")
	}

	switch {
	case snapshot.AgentSocketReady && containsFix(summary, "service"):
		fixed("Agent socket", config.DefaultPaths().AgentSocket())
	case snapshot.AgentSocketReady:
		pass("Agent socket", config.DefaultPaths().AgentSocket())
	default:
		fail("Agent socket not responding", config.DefaultPaths().AgentSocket())
	}

	switch {
	case snapshot.IPCSocketReady && containsFix(summary, "service"):
		fixed("IPC socket", config.DefaultPaths().CtlSocket())
	case snapshot.IPCSocketReady:
		pass("IPC socket", config.DefaultPaths().CtlSocket())
	default:
		fail("IPC socket not responding", config.DefaultPaths().CtlSocket())
	}
}

func renderDoctorSSH(snapshot readiness.Snapshot, summary readiness.RepairSummary, paths config.Paths) {
	switch {
	case snapshot.SSHEnabled && containsFix(summary, "ssh"):
		fixed("SSH agent", "Forged SSH config configured")
	case snapshot.SSHEnabled:
		pass("SSH agent", "Forged SSH config is configured")
	default:
		fail("SSH agent", "Forged SSH include not configured. Run: forged doctor --fix")
	}

	switch {
	case snapshot.ManagedConfigReady && containsFix(summary, "ssh"):
		fixed("SSH config", paths.SSHManagedConfig())
	case snapshot.ManagedConfigReady:
		pass("SSH config", paths.SSHManagedConfig())
	default:
		fail("SSH config", "missing Forged-managed SSH config. Run: forged doctor --fix")
	}
}

func renderDoctorIdentityOwner(snapshot readiness.Snapshot) {
	switch {
	case snapshot.IdentityAgentOwner.IsForged():
		pass("IdentityAgent owner", "Forged")
	case snapshot.IdentityAgentOwner.Name == "None":
		warn("IdentityAgent owner", "no active IdentityAgent is configured")
	case snapshot.IdentityAgentOwner.Name == "":
		warn("IdentityAgent owner", "could not inspect the active ssh configuration")
	default:
		detail := snapshot.IdentityAgentOwner.Name
		if snapshot.IdentityAgentOwner.Path != "" {
			detail += " (" + snapshot.IdentityAgentOwner.Path + ")"
		}
		if snapshot.SSHEnabled {
			warn("IdentityAgent owner", detail+" currently resolves first")
		} else {
			warn("IdentityAgent owner", detail)
		}
	}
}

func renderDoctorService(snapshot readiness.Snapshot) {
	switch {
	case snapshot.Service.Installed && snapshot.Service.ConfigValid:
		pass("System service", "installed")
	case snapshot.Service.Installed:
		fail("System service", snapshot.Service.Detail)
	default:
		warn("System service", "not installed (daemon won't auto-start)")
	}
}

func renderDoctorLogin(snapshot readiness.Snapshot) {
	if snapshot.LoggedIn {
		pass("Cloud credentials", "logged in")
		return
	}
	warn("Cloud credentials", "not logged in (sync disabled)")
}

func doctorIssueCount(snapshot readiness.Snapshot) int {
	issues := 0
	if !snapshot.VaultExists {
		issues++
	}
	if !snapshot.ConfigExists {
		issues++
	}
	if !snapshot.Service.Running {
		issues++
	}
	if !snapshot.AgentSocketReady {
		issues++
	}
	if !snapshot.IPCSocketReady {
		issues++
	}
	if !snapshot.SSHEnabled {
		issues++
	}
	if !snapshot.ManagedConfigReady {
		issues++
	}
	if snapshot.Service.Installed && !snapshot.Service.ConfigValid {
		issues++
	}
	return issues
}

func containsFix(summary readiness.RepairSummary, target string) bool {
	for _, item := range summary.Fixed {
		if item == target {
			return true
		}
	}
	return false
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "auto-fix issues where possible")
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
