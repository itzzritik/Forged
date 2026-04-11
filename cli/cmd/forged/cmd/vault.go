package cmd

import (
	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock vault and clear keys from memory",
	RunE:  notImplemented("lock"),
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock vault",
	RunE:  notImplemented("unlock"),
}
