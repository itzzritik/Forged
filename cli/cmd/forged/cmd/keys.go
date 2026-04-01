package cmd

import "github.com/spf13/cobra"

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Import a key from file",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented("add"),
}

var generateCmd = &cobra.Command{
	Use:   "generate <name>",
	Short: "Generate a new Ed25519 key pair",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented("generate"),
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all keys",
	RunE:  notImplemented("list"),
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a key",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented("remove"),
}

var exportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export public key to stdout",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented("export"),
}

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a key",
	Args:  cobra.ExactArgs(2),
	RunE:  notImplemented("rename"),
}
