package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var isInteractiveTerminal = terminalIsInteractive

func terminalIsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func shouldLaunchBareForged(args []string) bool {
	return len(args) == 0 && isInteractiveTerminal()
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
		RestoreVault: func(password []byte) error {
			return readiness.RestoreLinkedVault(paths, password)
		},
		StartLogin: func(server string, progress func(actions.LoginProgress)) (actions.LoginSession, error) {
			return actions.BeginLoginWithProgress(server, actions.OpenBrowser, progress)
		},
		SaveCredentials: func(creds actions.AccountCredentials) error { return actions.SaveCredentials(paths, creds) },
		TriggerSync:     func() error { return actions.TriggerSync(paths) },
		LoadSnapshot:    engine.Assess,
		LoadStatus: func() (tui.RuntimeStatus, error) {
			resp, err := ipc.NewClient(paths.CtlSocket()).Call(ipc.CmdStatus, nil)
			if err != nil {
				return tui.RuntimeStatus{}, err
			}
			var status struct {
				Sensitive *struct {
					Unlocked *bool `json:"unlocked"`
				} `json:"sensitive"`
				Sync struct {
					Dirty                bool      `json:"dirty"`
					LastErr              string    `json:"last_error"`
					Linked               bool      `json:"linked"`
					Syncing              bool      `json:"syncing"`
					LastSuccessfulPullAt time.Time `json:"last_successful_pull_at"`
					LastSuccessfulPushAt time.Time `json:"last_successful_push_at"`
				} `json:"sync"`
			}
			if err := json.Unmarshal(resp.Data, &status); err != nil {
				return tui.RuntimeStatus{}, err
			}
			runtimeStatus := tui.RuntimeStatus{
				Syncing:              status.Sync.Syncing,
				Dirty:                status.Sync.Dirty,
				Linked:               status.Sync.Linked,
				LastSuccessfulPullAt: status.Sync.LastSuccessfulPullAt,
				LastSuccessfulPushAt: status.Sync.LastSuccessfulPushAt,
				Error:                status.Sync.LastErr,
				SensitiveReported:    status.Sensitive != nil && status.Sensitive.Unlocked != nil,
			}
			if status.Sensitive != nil && status.Sensitive.Unlocked != nil {
				runtimeStatus.Unlocked = *status.Sensitive.Unlocked
				runtimeStatus.SensitiveKnown = true
			}
			return runtimeStatus, nil
		},
		ProbeSensitive: func() (tui.SensitiveState, error) {
			client := ipc.NewClient(paths.CtlSocket())

			resp, err := client.Call(ipc.CmdStatus, nil)
			if err != nil {
				return tui.SensitiveState{}, err
			}
			var status struct {
				Sensitive *struct {
					Unlocked *bool `json:"unlocked"`
				} `json:"sensitive"`
			}
			if err := json.Unmarshal(resp.Data, &status); err != nil {
				return tui.SensitiveState{}, err
			}
			if status.Sensitive != nil && status.Sensitive.Unlocked != nil {
				return tui.SensitiveState{Unlocked: *status.Sensitive.Unlocked, Known: true}, nil
			}

			listResp, err := client.Call(ipc.CmdList, nil)
			if err != nil {
				return tui.SensitiveState{}, err
			}
			var list struct {
				Keys []struct {
					Name string `json:"name"`
				} `json:"keys"`
			}
			if err := json.Unmarshal(listResp.Data, &list); err != nil {
				return tui.SensitiveState{}, err
			}
			if len(list.Keys) == 0 {
				return tui.SensitiveState{}, nil
			}

			_, err = client.Call(ipc.CmdView, map[string]any{
				"name": list.Keys[0].Name,
				"full": true,
			})
			switch {
			case err == nil:
				return tui.SensitiveState{Unlocked: true, Known: true}, nil
			case strings.Contains(err.Error(), "sensitive private-key access requires authentication"):
				return tui.SensitiveState{Unlocked: false, Known: true}, nil
			default:
				return tui.SensitiveState{}, err
			}
		},
		LockSensitive:   func() error { return actions.LockSensitive(paths) },
		UnlockSensitive: func(password []byte) (actions.UnlockResult, error) { return actions.UnlockSensitive(paths, password) },
		ChangePassword: func(currentPassword []byte, newPassword []byte) (actions.ChangePasswordResult, error) {
			return actions.ChangePassword(paths, currentPassword, newPassword)
		},
		LoadSigningStatus: func() (actions.CommitSigningStatus, error) { return actions.LoadCommitSigningStatus(paths) },
		EnableSSHAgent:    func() error { return actions.EnableSSHAgent(paths) },
		DisableSSHAgent:   func() error { return actions.DisableSSHAgent(paths) },
		EnableCommitSigning: func(name string) (actions.CommitSigningStatus, error) {
			return actions.EnableCommitSigning(paths, name)
		},
		DisableCommitSigning: func() (actions.CommitSigningStatus, error) {
			return actions.DisableCommitSigning(paths)
		},
		CopyText:      copyTextToClipboard,
		OpenLink:      openLinkInBrowser,
		DefaultServer: ipc.DefaultAPIServer,
		AppVersion:    version,
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
