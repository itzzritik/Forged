package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/ipc"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/readiness"
	"github.com/itzzritik/forged/cli/internal/sensitiveauth"
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
	clipboard := &clipboardManager{}

	_, err := tui.Run(intent, tui.Dependencies{
		Repair:      engine.Run,
		CreateVault: func(password []byte) error { return createLocalVault(paths, password) },
		RestoreVault: func(password []byte) error {
			return readiness.RestoreLinkedVault(paths, password)
		},
		StartLogin: func(ctx context.Context, server string, progress func(actions.LoginProgress)) (actions.LoginSession, error) {
			return actions.BeginLoginWithProgressContext(ctx, server, nil, progress)
		},
		SaveCredentials: func(creds actions.AccountCredentials) error { return actions.SaveCredentials(paths, creds) },
		TriggerSync:     func() error { return actions.TriggerSync(paths) },
		LoadStatus: func() (tui.RuntimeStatus, error) {
			return actions.LoadRuntimeStatus(paths)
		},
		LoadSecurityState: func() (tui.SecurityState, error) {
			return actions.LoadSecurityState(paths)
		},
		SetMasterPasswordInterval: func(value string) error {
			return actions.SetMasterPasswordInterval(paths, value)
		},
		HasLocalUnlockTrust: func() bool {
			return sensitiveauth.HasLocalEnrollment(paths)
		},
		UnlockSensitiveLaunch: func(ctx context.Context, password []byte, force bool) (actions.UnlockResult, error) {
			return actions.UnlockSensitiveLaunch(ctx, paths, password, force)
		},
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
		LoadSSHRoutingDebug: func() (actions.SSHRoutingDebug, error) {
			return actions.LoadSSHRoutingDebug(paths)
		},
		ClearSSHRoute: func(target string) error {
			return actions.ClearSSHRoute(paths, target)
		},
		ClearAllSSHRoutes: func() error {
			return actions.ClearAllSSHRoutes(paths)
		},
		CopyText:          clipboard.CopyText,
		CopySensitiveText: clipboard.CopySensitiveText,
		CloseClipboard:    clipboard.Close,
		OpenLink:          openLinkInBrowser,
		LogError: func(event actions.DiagnosticErrorEvent) {
			_ = actions.AppendDiagnosticError(paths, event)
		},
		DefaultServer: ipc.DefaultAPIServer,
		AppVersion:    version,
	})
	return err
}

func createLocalVault(paths config.Paths, password []byte) error {
	if _, err := os.Stat(paths.VaultFile()); err == nil {
		return fmt.Errorf("Vault already exists at %s", paths.VaultFile())
	}

	v, _, err := createVaultAtPaths(paths, password)
	if err != nil {
		return err
	}
	v.Close()
	return nil
}

const clipboardCommandTimeout = 5 * time.Second

const clipboardCloseAttempts = 3

type clipboardBackend struct {
	write     []string
	read      []string
	native    bool
	sensitive bool
}

type clipboardManager struct {
	mu     sync.Mutex
	closed bool
	nextID uint64
	active *activeClipboardLease
}

type activeClipboardLease struct {
	id      uint64
	backend clipboardBackend
	digest  [sha256.Size]byte
}

type commandClipboardLease struct {
	manager *clipboardManager
	id      uint64
}

func (l commandClipboardLease) ClearIfUnchanged() (bool, error) {
	return l.manager.clearIfUnchanged(l.id)
}

func (m *clipboardManager) CopyText(value string) error {
	data := []byte(value)
	defer clear(data)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("Clipboard is closed")
	}
	_, err := copyToClipboard(data, false, false)
	if err == nil {
		m.active = nil
	}
	return err
}

func (m *clipboardManager) CopySensitiveText(value string) (tui.SensitiveClipboardLease, error) {
	data := []byte(value)
	defer clear(data)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("Clipboard is closed")
	}
	backend, err := copyToClipboard(data, true, true)
	if err != nil {
		return nil, err
	}
	m.nextID++
	m.active = &activeClipboardLease{
		id:      m.nextID,
		backend: backend,
		digest:  clipboardDigest(data),
	}
	return commandClipboardLease{manager: m, id: m.nextID}, nil
}

func (m *clipboardManager) clearIfUnchanged(id uint64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clearIfUnchangedLocked(id)
}

func (m *clipboardManager) clearIfUnchangedLocked(id uint64) (bool, error) {
	if m.active == nil || m.active.id != id {
		return false, nil
	}
	current, err := readClipboard(m.active.backend)
	if err != nil {
		return false, fmt.Errorf("reading clipboard: %w", err)
	}
	defer clear(current)
	if clipboardDigest(current) != m.active.digest {
		m.active = nil
		return false, nil
	}
	if err := writeClipboard(m.active.backend, nil); err != nil {
		return false, fmt.Errorf("clearing clipboard: %w", err)
	}
	m.active = nil
	return true, nil
}

func (m *clipboardManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	if m.active == nil {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < clipboardCloseAttempts; attempt++ {
		if _, err := m.clearIfUnchangedLocked(m.active.id); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < clipboardCloseAttempts-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("Couldn't clear private key from clipboard; clear it manually: %w", lastErr)
}

func clipboardDigest(value []byte) [sha256.Size]byte {
	normalized := bytes.ReplaceAll(value, []byte("\r\n"), []byte("\n"))
	defer clear(normalized)
	return sha256.Sum256(normalized)
}

func copyToClipboard(value []byte, sensitive bool, requireRead bool) (clipboardBackend, error) {
	backends, err := clipboardBackends(sensitive)
	if err != nil {
		return clipboardBackend{}, err
	}
	var lastErr error
	for _, backend := range backends {
		if !backend.native {
			if _, err := exec.LookPath(backend.write[0]); err != nil {
				lastErr = err
				continue
			}
			if requireRead {
				if _, err := exec.LookPath(backend.read[0]); err != nil {
					lastErr = err
					continue
				}
			}
		}
		if err := writeClipboard(backend, value); err != nil {
			lastErr = err
			continue
		}
		return backend, nil
	}
	if lastErr != nil {
		return clipboardBackend{}, fmt.Errorf("Copy failed: %w", lastErr)
	}
	return clipboardBackend{}, fmt.Errorf("No clipboard helper is available")
}

func clipboardBackends(sensitive bool) ([]clipboardBackend, error) {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardBackend{{write: []string{"pbcopy"}, read: []string{"pbpaste"}}}, nil
	case "linux":
		backends := make([]clipboardBackend, 0, 4)
		if sensitive {
			backends = append(backends, clipboardBackend{
				write: []string{"wl-copy", "--sensitive", "--type", "text/plain"},
				read:  []string{"wl-paste", "--no-newline", "--type", "text"},
			})
		}
		return append(backends,
			clipboardBackend{write: []string{"wl-copy", "--type", "text/plain"}, read: []string{"wl-paste", "--no-newline", "--type", "text"}},
			clipboardBackend{write: []string{"xclip", "-selection", "clipboard", "-in"}, read: []string{"xclip", "-selection", "clipboard", "-out"}},
			clipboardBackend{write: []string{"xsel", "--clipboard", "--input"}, read: []string{"xsel", "--clipboard", "--output"}},
		), nil
	case "windows":
		return []clipboardBackend{{native: true, sensitive: sensitive}}, nil
	default:
		return nil, fmt.Errorf("Clipboard copy is not supported on %s", runtime.GOOS)
	}
}

func writeClipboard(backend clipboardBackend, value []byte) error {
	if backend.native {
		return platform.ClipboardWriteText(value, backend.sensitive)
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipboardCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, backend.write[0], backend.write[1:]...)
	cmd.Stdin = bytes.NewReader(value)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func readClipboard(backend clipboardBackend) ([]byte, error) {
	if backend.native {
		return platform.ClipboardReadText()
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipboardCommandTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, backend.read[0], backend.read[1:]...).Output()
	if err != nil {
		clear(output)
		return nil, err
	}
	return output, nil
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
		return fmt.Errorf("Opening links is not supported on %s", runtime.GOOS)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
