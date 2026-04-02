package cmd

import (
	"crypto/rand"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/argon2"
)

var signCmd = &cobra.Command{
	Use:    "sign",
	Short:  "Git signing helper (called by git)",
	Hidden: true,
	RunE:   notImplemented("sign"),
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail daemon logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		logPath := paths.LogFile()
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			return fmt.Errorf("no log file found at %s", logPath)
		}
		return syscall.Exec("/usr/bin/tail", []string{"tail", "-f", logPath}, os.Environ())
	},
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Test Argon2id speed and recommend parameters",
	RunE: func(cmd *cobra.Command, args []string) error {
		salt := make([]byte, 32)
		rand.Read(salt)
		password := []byte("benchmark-test-password")

		configs := []struct {
			name   string
			time   uint32
			memory uint32
			threads uint8
		}{
			{"light (32MB, 2 iter)", 2, 32 * 1024, 4},
			{"default (64MB, 3 iter)", 3, 64 * 1024, 4},
			{"heavy (128MB, 4 iter)", 4, 128 * 1024, 4},
		}

		fmt.Println("Benchmarking Argon2id key derivation...")
		fmt.Println()

		type result struct {
			name     string
			duration time.Duration
		}
		var results []result

		for _, c := range configs {
			start := time.Now()
			argon2.IDKey(password, salt, c.time, c.memory, c.threads, 32)
			dur := time.Since(start)
			results = append(results, result{c.name, dur})
			fmt.Printf("  %-25s %s\n", c.name, dur.Round(time.Millisecond))
		}

		if jsonOutput {
			out := make([]map[string]any, len(results))
			for i, r := range results {
				out[i] = map[string]any{"name": r.name, "duration_ms": r.duration.Milliseconds()}
			}
			return printOutput(out)
		}

		fmt.Println()
		fmt.Println("Recommendation: default (64MB, 3 iter) is good for most machines.")
		fmt.Println("Use heavy if derivation takes under 500ms on your hardware.")
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOutput {
			return printOutput(map[string]string{
				"version": version,
				"commit":  commit,
			})
		}
		fmt.Printf("forged %s (%s)\n", version, commit)
		return nil
	},
}
