package tui

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/platform"
	"github.com/itzzritik/forged/cli/internal/tui/core"
)

type (
	Deps           = core.Deps
	ClipboardLease = core.ClipboardLease
	RuntimeStatus  = actions.RuntimeStatus
	SecurityState  = actions.SecurityState
)

var ErrTerminalClipboard = core.ErrTerminalClipboard

type Intent struct{ Doctor bool }

func DashboardIntent() Intent { return Intent{} }

func DoctorIntent() Intent { return Intent{Doctor: true} }

func programOptions() []tea.ProgramOption {
	// SSH drops COLORTERM, so a truecolor client otherwise looks like 256 colors here.
	if platform.OverSSH() && os.Getenv("COLORTERM") == "" && os.Getenv("NO_COLOR") == "" && strings.Contains(os.Getenv("TERM"), "256color") {
		return []tea.ProgramOption{tea.WithColorProfile(colorprofile.TrueColor)}
	}
	return nil
}

func Run(intent Intent, deps Deps) error {
	if err := deps.Validate(); err != nil {
		return err
	}
	// A password change stops the daemon; it must finish and restart it even if the TUI exits first.
	var passwordChange sync.Mutex
	change := deps.ChangePassword
	deps.ChangePassword = func(current, next []byte) (actions.ChangePasswordResult, error) {
		passwordChange.Lock()
		defer passwordChange.Unlock()
		return change(current, next)
	}
	// Library warnings would print over the screen; send them to the TUI log instead.
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(tuiLogWriter(deps.LogError), nil)))
	defer slog.SetDefault(previous)
	// The first check runs ssh -V; warm it up off the UI thread.
	go platform.SSHRoutingSupported()
	a := newApp(intent, deps, config.DefaultPaths())
	p := tea.NewProgram(a, programOptions()...)
	quitOnHangup(p)
	_, err := p.Run()
	passwordChange.Lock()
	a.shutdown()
	if err != nil {
		err = fmt.Errorf("running TUI: %w", err)
		a.st.Reporter.Report("app", "tui.run", err)
	}
	if cerr := deps.CloseClipboard(); cerr != nil {
		cerr = fmt.Errorf("closing clipboard: %w", cerr)
		a.st.Reporter.Report("app", "clipboard.close", cerr)
		err = errors.Join(err, cerr)
	}
	return err
}

// Closing the terminal sends SIGHUP; quit normally so a copied private key is still cleared.
func quitOnHangup(p *tea.Program) {
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)
	go func() {
		<-hangup
		p.Quit()
	}()
}

type tuiLogWriter func(actions.DiagnosticErrorEvent)

func (w tuiLogWriter) Write(p []byte) (int, error) {
	w(actions.DiagnosticErrorEvent{Route: "app", Action: "log", Message: strings.TrimSpace(string(p))})
	return len(p), nil
}
