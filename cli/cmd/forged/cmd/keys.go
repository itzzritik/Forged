package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	Use:   "generate [name]",
	Short: "Generate a new Ed25519 key pair",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		comment, _ := cmd.Flags().GetString("comment")

		if name == "" && !jsonOutput {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("  A short name to identify this key (e.g. github, work, prod-server)")
			fmt.Print("  Name: ")
			line, _ := reader.ReadString('\n')
			name = strings.TrimSpace(line)
			if name == "" {
				return fmt.Errorf("key name is required")
			}

			if comment == "" {
				fmt.Println()
				fmt.Println("  A label attached to the public key (e.g. your email or username)")
				fmt.Print("  Label: ")
				line, _ = reader.ReadString('\n')
				comment = strings.TrimSpace(line)
			}
		}

		if name == "" {
			return fmt.Errorf("key name is required")
		}

		resp, err := ctlClient().Call("generate", map[string]string{
			"name":    name,
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

		pub := result["public_key"]
		if comment != "" {
			pub = pub + " " + comment
		}

		fmt.Println()
		fmt.Printf("  Key:         %s\n", result["name"])
		fmt.Printf("  Type:        %s\n", result["type"])
		fmt.Printf("  Fingerprint: %s\n", result["fingerprint"])
		fmt.Println()
		fmt.Println("  Public key (add this to GitHub/GitLab/server):")
		fmt.Println()
		fmt.Printf("    %s\n", pub)
		fmt.Println()

		fmt.Println("  Add this public key to:")
		fmt.Println("    GitHub:  Settings > SSH Keys > New SSH Key")
		fmt.Println("    GitLab:  Preferences > SSH Keys > Add new key")
		fmt.Println("    Server:  ssh-copy-id or append to ~/.ssh/authorized_keys")
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
