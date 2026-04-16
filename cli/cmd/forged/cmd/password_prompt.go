package cmd

import (
	"fmt"
	"os"
)

func promptMasterPassword(reason string) ([]byte, error) {
	if reason != "" {
		fmt.Fprintln(os.Stderr, reason)
	}
	return getPassword()
}
