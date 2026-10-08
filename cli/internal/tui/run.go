package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/itzzritik/forged/cli/internal/actions"
	"github.com/itzzritik/forged/cli/internal/config"
	"github.com/itzzritik/forged/cli/internal/tui/core"
)

type (
	Deps           = core.Deps
	ClipboardLease = core.ClipboardLease
	RuntimeStatus  = actions.RuntimeStatus
	SecurityState  = actions.SecurityState
)

type Intent struct{ Doctor bool }

func DashboardIntent() Intent { return Intent{} }

func DoctorIntent() Intent { return Intent{Doctor: true} }

func Run(intent Intent, deps Deps) error {
	if err := deps.Validate(); err != nil {
		return err
	}
	a := newApp(intent, deps, config.DefaultPaths())
	_, err := tea.NewProgram(a).Run()
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
