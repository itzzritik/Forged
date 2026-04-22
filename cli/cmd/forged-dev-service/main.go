package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("forged-dev-service", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	binary := fs.String("binary", "", "path to the repo forged binary")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  forged-dev-service --binary <path> install")
		fmt.Fprintln(os.Stderr, "  forged-dev-service stop")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	switch fs.Arg(0) {
	case "install":
		return install(*binary)
	case "stop":
		return stop()
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
}

func install(binary string) error {
	if binary == "" {
		return fmt.Errorf("missing --binary path")
	}

	absBinary, err := filepath.Abs(binary)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}
	if _, err := os.Stat(absBinary); err != nil {
		return fmt.Errorf("checking forged binary %s: %w", absBinary, err)
	}

	paths := config.DefaultPaths()
	if _, err := os.Stat(paths.VaultFile()); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("vault not found at %s; run ./bin/forged first", paths.VaultFile())
		}
		return fmt.Errorf("checking vault: %w", err)
	}

	runtime := daemon.RuntimeSpec{
		Binary: absBinary,
		Args:   []string{"daemon"},
	}
	if err := daemon.EnsureService(paths, runtime); err != nil {
		return err
	}

	fmt.Printf("Forged service now points to %s\n", absBinary)
	return nil
}

func stop() error {
	if !daemon.ServiceInstalled() {
		fmt.Println("Forged service is not installed")
		return nil
	}
	if err := daemon.StopService(); err != nil {
		return err
	}
	fmt.Println("Forged service stopped")
	return nil
}
