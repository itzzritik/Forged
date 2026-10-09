package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	a := newApp(intent, deps, config.DefaultPaths())
	_, err := tea.NewProgram(a, programOptions()...).Run()
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
