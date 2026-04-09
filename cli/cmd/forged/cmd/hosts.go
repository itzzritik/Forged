package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{
	Use:   "host <key-name> <patterns...>",
	Short: "Map a key to host patterns",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call(ipc.CmdHost, map[string]any{
			"key_name": args[0],
			"patterns": args[1:],
		})
		if err != nil {
			return err
		}
		fmt.Printf("Mapped %s to %v\n", args[0], args[1:])
		return nil
	},
}

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "List all host-key mappings",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call(ipc.CmdHosts, nil)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result struct {
			Mappings []struct {
				KeyName string `json:"key_name"`
				Rules   []struct {
					Match string `json:"match"`
					Type  string `json:"type"`
				} `json:"rules"`
			} `json:"mappings"`
		}
		json.Unmarshal(resp.Data, &result)

		if len(result.Mappings) == 0 {
			fmt.Println("No host mappings configured")
			return nil
		}
		for _, m := range result.Mappings {
			for _, r := range m.Rules {
				fmt.Printf("  %s\t%s\t(%s)\n", m.KeyName, r.Match, r.Type)
			}
		}
		return nil
	},
}

var unhostCmd = &cobra.Command{
	Use:   "unhost <key-name> <pattern>",
	Short: "Remove a host mapping",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call(ipc.CmdUnhost, map[string]string{
			"key_name": args[0],
			"pattern":  args[1],
		})
		if err != nil {
			return err
		}
		fmt.Printf("Removed %s from %s\n", args[1], args[0])
		return nil
	},
}
