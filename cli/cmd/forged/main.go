package main

import (
	"os"

	"github.com/itzzritik/forged/cli/cmd/forged/cmd"
	"github.com/itzzritik/forged/cli/internal/platform"
)

func main() {
	platform.EnsureHome()
	if err := cmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
