package cmd

import (
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status, key count, sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		snapshot, err := readiness.New(paths).Assess()
		if err != nil {
			return err
		}

		if jsonOutput {
			return printOutput(map[string]any{
				"state":              snapshot.State,
				"running":            snapshot.Service.Running,
				"key_count":          snapshot.KeyCount,
				"logged_in":          snapshot.LoggedIn,
				"ipc_socket_ready":   snapshot.IPCSocketReady,
				"agent_socket_ready": snapshot.AgentSocketReady,
				"ipc_socket":         paths.CtlSocket(),
				"agent_socket":       paths.AgentSocket(),
				"pid":                snapshot.DaemonPID,
			})
		}

		fmt.Printf("State:  %s\n", snapshot.State)
		if snapshot.DaemonPID > 0 {
			fmt.Printf("Daemon: PID %d\n", snapshot.DaemonPID)
		}
		if snapshot.Service.Detail != "" && snapshot.State != readiness.StateReady && snapshot.State != readiness.StateReadyEmpty {
			fmt.Printf("Issue:  %s\n", snapshot.Service.Detail)
		}
		fmt.Printf("Keys:   %d loaded\n", snapshot.KeyCount)
		if snapshot.LoggedIn {
			fmt.Println("Sync:   linked")
		} else {
			fmt.Println("Sync:   not linked")
		}
		if snapshot.State == readiness.StateDegraded || snapshot.State == readiness.StateBlocked {
			fmt.Println("Action: run `forged` to repair interactively or `forged doctor --fix`")
		}
		fmt.Printf("IPC:    %s\n", paths.CtlSocket())
		fmt.Printf("Agent:  %s\n", paths.AgentSocket())
		return nil
	},
}

func statusRun(cmd *cobra.Command, args []string) error {
	return statusCmd.RunE(cmd, args)
}
