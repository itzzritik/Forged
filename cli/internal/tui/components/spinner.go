package components

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/itzzritik/forged/cli/internal/tui/theme"
)

func NewSpinner() spinner.Model {
	model := spinner.New()
	model.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    time.Second / 12,
	}
	model.Style = theme.Spinner
	return model
}
