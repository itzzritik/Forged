package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/daemon"
	"github.com/itzzritik/forged/cli/internal/vault"
	"golang.org/x/term"
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
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("vault not found at %s; run ./bin/forged or ./bin/forged doctor --fix first", paths.VaultFile())
		}
		return fmt.Errorf("checking vault: %w", err)
	}

	password, err := resolveMasterPassword(paths)
	if err != nil {
		return err
	}

	runtime := daemon.RuntimeSpec{
		Binary: absBinary,
		Args:   []string{"daemon"},
	}
	if err := daemon.EnsureService(paths, daemon.ServiceCredentials{
		MasterPassword: string(password),
	}, runtime); err != nil {
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

func resolveMasterPassword(paths config.Paths) ([]byte, error) {
	if daemon.ServiceInstalled() {
		if password, err := daemon.ReadInstalledServicePassword(); err == nil && password != "" {
			if err := vault.VerifyPassword(paths.VaultFile(), []byte(password)); err == nil {
				return []byte(password), nil
			}
		}
	}

	if env := os.Getenv("FORGED_MASTER_PASSWORD"); env != "" {
		if err := vault.VerifyPassword(paths.VaultFile(), []byte(env)); err != nil {
			return nil, fmt.Errorf("verifying FORGED_MASTER_PASSWORD: %w", err)
		}
		return []byte(env), nil
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("master password required; rerun interactively or set FORGED_MASTER_PASSWORD")
	}

	fmt.Fprint(os.Stderr, "Master password: ")
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading master password: %w", err)
	}
	if err := vault.VerifyPassword(paths.VaultFile(), password); err != nil {
		return nil, fmt.Errorf("verifying master password: %w", err)
	}
	return password, nil
}
