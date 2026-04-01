package main

import (
	"os"

	"github.com/forgedkeys/forged/cli/cmd/forged/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
