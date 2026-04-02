package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/spf13/cobra"
)

func ctlClient() *ipc.Client {
	return ipc.NewClient(config.DefaultPaths().CtlSocket())
}

var addCmd = &cobra.Command{
	Use:   "add <name> --file <path>",
	Short: "Import a key from file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading key file: %w", err)
		}
		comment, _ := cmd.Flags().GetString("comment")

		resp, err := ctlClient().Call("add", map[string]string{
			"name":        args[0],
			"private_key": string(data),
			"comment":     comment,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result map[string]string
		json.Unmarshal(resp.Data, &result)
		fmt.Printf("Added %s (%s)\n  %s\n", result["name"], result["type"], result["fingerprint"])
		return nil
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate <name>",
	Short: "Generate a new Ed25519 key pair",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")

		resp, err := ctlClient().Call("generate", map[string]string{
			"name":    args[0],
			"comment": comment,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result map[string]string
		json.Unmarshal(resp.Data, &result)
		fmt.Printf("Generated %s (%s)\n  %s\n  %s\n", result["name"], result["type"], result["fingerprint"], result["public_key"])
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call("list", nil)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result struct {
			Keys []struct {
				Name        string `json:"name"`
				Type        string `json:"type"`
				Fingerprint string `json:"fingerprint"`
			} `json:"keys"`
		}
		json.Unmarshal(resp.Data, &result)

		if len(result.Keys) == 0 {
			fmt.Println("No keys in vault")
			return nil
		}
		for _, k := range result.Keys {
			fmt.Printf("  %s\t%s\t%s\n", k.Name, k.Type, k.Fingerprint)
		}
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call("remove", map[string]string{"name": args[0]})
		if err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", args[0])
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export public key to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := ctlClient().Call("export", map[string]string{"name": args[0]})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printOutput(json.RawMessage(resp.Data))
		}
		var result map[string]string
		json.Unmarshal(resp.Data, &result)
		fmt.Println(result["public_key"])
		return nil
	},
}

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a key",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := ctlClient().Call("rename", map[string]string{
			"old_name": args[0],
			"new_name": args[1],
		})
		if err != nil {
			return err
		}
		fmt.Printf("Renamed %s → %s\n", args[0], args[1])
		return nil
	},
}

func init() {
	addCmd.Flags().StringP("file", "f", "", "path to private key file")
	addCmd.Flags().StringP("comment", "c", "", "key comment")
	generateCmd.Flags().StringP("comment", "c", "", "key comment")
}
