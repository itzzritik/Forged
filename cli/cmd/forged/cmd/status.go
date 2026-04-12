package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status, key count, sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		pid, running := daemon.IsRunning(paths)

		if !running {
			if jsonOutput {
				return printOutput(map[string]any{
					"running": false,
					"socket":  paths.AgentSocket(),
				})
			}
			fmt.Println("Daemon: not running")
			fmt.Printf("Socket: %s\n", paths.AgentSocket())
			return nil
		}

		resp, err := ctlClient().Call(ipc.CmdStatus, nil)
		if err != nil {
			if jsonOutput {
				return printOutput(map[string]any{
					"running": true,
					"pid":     pid,
					"socket":  paths.AgentSocket(),
				})
			}
			fmt.Printf("Daemon: running (PID %d)\n", pid)
			fmt.Printf("Socket: %s\n", paths.AgentSocket())
			return nil
		}

		if jsonOutput {
			var data map[string]any
			json.Unmarshal(resp.Data, &data)
			data["running"] = true
			data["socket"] = paths.AgentSocket()
			return printOutput(data)
		}

		var data struct {
			PID      int `json:"pid"`
			KeyCount int `json:"key_count"`
			Sync     struct {
				Dirty                  bool   `json:"dirty"`
				LastError              string `json:"last_error"`
				LastKnownServerVersion int64  `json:"last_known_server_version"`
				Linked                 bool   `json:"linked"`
				LinkedUserID           string `json:"linked_user_id"`
				ServerURL              string `json:"server_url"`
			} `json:"sync"`
		}
		json.Unmarshal(resp.Data, &data)

		fmt.Printf("Daemon: running (PID %d)\n", data.PID)
		fmt.Printf("Keys:   %d loaded\n", data.KeyCount)
		if data.Sync.Linked {
			syncState := "clean"
			if data.Sync.Dirty {
				syncState = "dirty"
			}
			fmt.Printf("Sync:   linked (%s), version %d\n", syncState, data.Sync.LastKnownServerVersion)
			if data.Sync.LastError != "" {
				fmt.Printf("Sync:   last error: %s\n", data.Sync.LastError)
			}
		} else {
			fmt.Println("Sync:   not linked")
		}
		fmt.Printf("Socket: %s\n", paths.AgentSocket())
		return nil
	},
}

func statusRun(cmd *cobra.Command, args []string) error {
	return statusCmd.RunE(cmd, args)
}
