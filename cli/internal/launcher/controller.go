package launcher

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/itzzritik/forged/cli/internal/readiness"
)

type ReadinessRunner interface {
	Assess() (readiness.Snapshot, error)
	Repair(readiness.Snapshot) (readiness.Snapshot, readiness.RepairSummary, error)
}

type ActionFunc func(readiness.Snapshot) (string, error)

type Dependencies struct {
	Readiness ReadinessRunner
	Actions   map[ActionID]ActionFunc
}

type Controller struct {
	deps Dependencies
}

func NewController(deps Dependencies) *Controller {
	return &Controller{deps: deps}
}

func (c *Controller) Run() error {
	flash := ""
	for {
		model := NewModel(func() (readiness.Snapshot, readiness.RepairSummary, error) {
			snapshot, err := c.deps.Readiness.Assess()
			if err != nil {
				return readiness.Snapshot{}, readiness.RepairSummary{}, err
			}
			return c.deps.Readiness.Repair(snapshot)
		}, flash)

		final, err := tea.NewProgram(model).Run()
		if err != nil {
			return err
		}

		out, ok := final.(*Model)
		if !ok {
			return fmt.Errorf("unexpected launcher model type %T", final)
		}
		if out.err != nil {
			return out.err
		}
		if out.cancelled || out.selected == "" {
			return nil
		}

		action := c.deps.Actions[out.selected]
		if action == nil {
			flash = errorStyle.Render("Unsupported action")
			continue
		}

		message, err := action(out.snapshot)
		if err != nil {
			flash = errorStyle.Render(err.Error())
			continue
		}
		if message != "" {
			flash = successStyle.Render(message)
		} else {
			flash = ""
		}
	}
}
