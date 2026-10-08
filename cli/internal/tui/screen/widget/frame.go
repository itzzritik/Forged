package widget

import (
	"github.com/itzzritik/forged/cli/internal/tui/core"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type Frame struct{ X0, R, W, Gap int }

func FrameFor(st *core.State, w int) Frame {
	x0 := 0
	if st.Width >= 60 {
		x0 = 2
	}
	gap := 0
	if st.Height >= 20 {
		gap = 1
	}
	return Frame{x0, w, w - x0, gap}
}

func (f Frame) Pad(s string) string { return ui.Repeat(" ", f.X0) + s }
