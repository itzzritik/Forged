package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
)

var isInteractiveTerminal = terminalIsInteractive

func shouldLaunchBareForged(args []string) bool {
	return len(args) == 0 && !jsonOutput && isInteractiveTerminal()
}

func runBareForged(cmd *cobra.Command) error {
	return runInteractiveIntent(tui.DashboardIntent())
}

func runInteractiveIntent(intent tui.Intent) error {
	paths := config.DefaultPaths()
	engine := readiness.New(paths)

	_, err := tui.Run(intent, tui.Dependencies{
		Repair:      engine.Run,
		CreateVault: func(password []byte) error { return createLocalVault(paths, password) },
		StartLogin: func(server string) (actions.LoginSession, error) {
			return actions.BeginLogin(server, actions.OpenBrowser)
		},
		SaveCredentials: func(creds actions.AccountCredentials) error { return actions.SaveCredentials(paths, creds) },
		CopyText:        copyTextToClipboard,
		OpenLink:        openLinkInBrowser,
		DefaultServer:   ipc.DefaultAPIServer,
		AppVersion:      version,
		CommitSigning:   commitSigningConfigured(),
	})
	return err
}

func createLocalVault(paths config.Paths, password []byte) error {
	if _, err := os.Stat(paths.VaultFile()); err == nil {
		return fmt.Errorf("vault already exists at %s", paths.VaultFile())
	}

	v, _, err := createVaultAtPaths(paths, password)
	if err != nil {
		return err
	}
	v.Close()
	return nil
}

func copyTextToClipboard(value string) error {
	var commands [][]string
	switch runtime.GOOS {
	case "darwin":
		commands = [][]string{{"pbcopy"}}
	case "linux":
		commands = [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "--clipboard", "--input"},
		}
	case "windows":
		commands = [][]string{{"clip"}}
	default:
		return fmt.Errorf("clipboard copy is not supported on %s", runtime.GOOS)
	}

	var lastErr error
	for _, argv := range commands {
		if _, err := exec.LookPath(argv[0]); err != nil {
			lastErr = err
			continue
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("copy failed: %w", lastErr)
	}
	return fmt.Errorf("no clipboard helper is available")
}

func openLinkInBrowser(url string) error {
	var argv []string
	switch runtime.GOOS {
	case "darwin":
		argv = []string{"open", url}
	case "linux":
		argv = []string{"xdg-open", url}
	case "windows":
		argv = []string{"rundll32", "url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("open-link is not supported on %s", runtime.GOOS)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func commitSigningConfigured() bool {
	signingKey := gitGlobalConfig("user.signingkey")
	gpgFormat := strings.ToLower(gitGlobalConfig("gpg.format"))
	signProgram := gitGlobalConfig("gpg.ssh.program")
	commitSign := strings.ToLower(gitGlobalConfig("commit.gpgsign"))

	if signingKey == "" || signProgram == "" {
		return false
	}
	if gpgFormat != "" && gpgFormat != "ssh" {
		return false
	}
	if commitSign != "" && commitSign != "true" {
		return false
	}
	return strings.Contains(signProgram, "forged-sign")
}

func gitGlobalConfig(key string) string {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
