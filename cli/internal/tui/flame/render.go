package flame

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/itzzritik/forged/cli/internal/tui/ui"
)

type ramp struct {
	version uint64
	fg, bg  [levels + 1]string
}

var cache ramp

func sgr(c color.Color, fg bool) string {
	r, g, b, _ := c.RGBA()
	k := 38
	if !fg {
		k = 48
	}
	return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", k, r>>8, g>>8, b>>8)
}

func colors() *ramp {
	if cache.version == ui.Version()+1 {
		return &cache
	}
	p := ui.P()
	stops := append([]color.Color{p.Bg}, p.Fire...)
	cs := lipgloss.Blend1D(levels+1, stops...)
	for i := 1; i <= levels; i++ {
		cache.fg[i], cache.bg[i] = sgr(cs[i], true), sgr(cs[i], false)
	}
	cache.version = ui.Version() + 1
	return &cache
}

func Render(f *Fire) []string {
	if f == nil {
		return nil
	}
	c := colors()
	lines := make([]string, f.rows)
	scale := func(v uint8) int { return min(levels, int(v)) }
	for row := 0; row < f.rows; row++ {
		var b strings.Builder
		lastFg, lastBg := -1, -1
		for x := 0; x < f.w; x++ {
			a := scale(f.out[(row*2)*f.w+x])
			d := scale(f.out[(row*2+1)*f.w+x])
			switch {
			case a == 0 && d == 0:
				if lastFg != 0 || lastBg != 0 {
					b.WriteString("\x1b[0m")
					lastFg, lastBg = 0, 0
				}
				b.WriteByte(' ')
				continue
			case a == 0:
				if lastBg != 0 {
					b.WriteString("\x1b[0m")
					lastFg, lastBg = -1, 0
				}
				if lastFg != d {
					b.WriteString(c.fg[d])
					lastFg = d
				}
				b.WriteString("▄")
				continue
			}
			if lastFg != a {
				b.WriteString(c.fg[a])
				lastFg = a
			}
			if d == 0 {
				if lastBg != 0 {
					b.WriteString("\x1b[49m")
					lastBg = 0
				}
			} else if lastBg != d {
				b.WriteString(c.bg[d])
				lastBg = d
			}
			b.WriteString("▀")
		}
		b.WriteString("\x1b[0m")
		lines[row] = b.String()
	}
	return lines
}

func Animate() bool {
	if os.Getenv("FORGED_ANIMATIONS") == "0" || ui.ASCII() || os.Getenv("NO_COLOR") != "" ||
		os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	return colorprofile.Detect(os.Stdout, os.Environ()) >= colorprofile.ANSI256
}
