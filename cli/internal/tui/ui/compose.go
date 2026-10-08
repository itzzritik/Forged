package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Layer struct {
	Content string
	X, Y    int
	Dim     bool
	Fill    color.Color
}

func Compose(w, h int, base string, layers ...Layer) string {
	c := lipgloss.NewCanvas(w, h)
	if base != "" {
		c.Compose(lipgloss.NewLayer(base))
	}
	for _, l := range layers {
		if l.Dim {
			dimCells(c, w, h)
		}
		c.Compose(lipgloss.NewCompositor(lipgloss.NewLayer(l.Content).X(l.X).Y(l.Y)))
		if l.Fill != nil {
			fillCells(c, l.X, l.Y, lipgloss.Width(l.Content), lipgloss.Height(l.Content), l.Fill)
		}
	}
	return c.Render()
}

func dimCells(c *lipgloss.Canvas, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := c.CellAt(x, y)
			if cell == nil {
				continue
			}
			cell.Style.Fg = P().DimText
			if cell.Style.Bg != nil {
				cell.Style.Bg = P().DimFill
			}
			cell.Style.Attrs = 0
		}
	}
}

func fillCells(c *lipgloss.Canvas, x0, y0, w, h int, bg color.Color) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if cell := c.CellAt(x, y); cell != nil && cell.Style.Bg == nil {
				cell.Style.Bg = bg
			}
		}
	}
}
