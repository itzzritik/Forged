package ui

import (
	"image/color"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

type Palette struct {
	Bg, Text, Muted, Faint, Line, LineFocus, Rule, Surface, Cap, Code color.Color
	Good, Warn, Danger, Steel, WarnBg, DimText, DimFill               color.Color
	Accent, Sel, OnAccent                                             color.Color
	Sweep                                                             [3]color.Color
	Fire                                                              []color.Color
	Spark                                                             [2]color.Color
}

func hex(s string) color.Color { return lipgloss.Color(s) }

func hexes(s ...string) []color.Color {
	out := make([]color.Color, len(s))
	for i, v := range s {
		out[i] = hex(v)
	}
	return out
}

var (
	mu      sync.RWMutex
	dark    = base(true)
	light   = base(false)
	isDark  atomic.Bool
	pinned  bool
	version atomic.Uint64
)

func base(d bool) Palette {
	if d {
		return Palette{
			Bg: hex("#111111"), Text: hex("#ECEBE9"), Muted: hex("#9C9B98"), Faint: hex("#605F5C"),
			Line: hex("#333333"), LineFocus: hex("#555452"), Rule: hex("#222222"), Surface: hex("#1A1A1A"),
			Cap: hex("#2A2A2A"), Code: hex("#171717"), Good: hex("#7CC98A"), Warn: hex("#F5B841"),
			Danger: hex("#F0503C"), Steel: hex("#8FB0D0"), WarnBg: hex("#211C12"), DimText: hex("#383837"),
			DimFill: hex("#151515"), Accent: hex("#FF7A1A"), OnAccent: hex("#160A04"),
			Sweep: [3]color.Color{hex("#F0412A"), hex("#FF7A1A"), hex("#FFC23D")},
			Fire:  hexes("#2B110A", "#4F1407", "#7E1C08", "#B32A0A", "#E2420E", "#FF6614", "#FF8A1F", "#FFAE33", "#FFCE6E", "#FFE9B3", "#FFF8E8"),
			Spark: [2]color.Color{hex("#FFB224"), hex("#FF6614")},
		}
	}
	return Palette{
		Bg: hex("#FFFFFF"), Text: hex("#18181A"), Muted: hex("#5F5E5B"), Faint: hex("#A3A29F"),
		Line: hex("#DBDAD7"), LineFocus: hex("#ABAAA6"), Rule: hex("#ECECEA"), Surface: hex("#FFFFFF"),
		Cap: hex("#ECEBE9"), Code: hex("#F5F5F4"), Good: hex("#15803D"), Warn: hex("#A16207"),
		Danger: hex("#B91C1C"), Steel: hex("#2F5F8A"), WarnBg: hex("#FBF0DC"), DimText: hex("#D9D8D5"),
		DimFill: hex("#F7F7F6"), Accent: hex("#D9480F"), OnAccent: hex("#FFFFFF"),
		Sweep: [3]color.Color{hex("#BF2618"), hex("#D9480F"), hex("#B97509")},
		Fire:  hexes("#FCE9D8", "#FAD0AE", "#F7B07F", "#F28C4F", "#EA6A2C", "#DB4F18", "#C63D10", "#AE320C"),
		Spark: [2]color.Color{hex("#B45309"), hex("#C2410C")},
	}
}

func init() {
	isDark.Store(true)
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FORGED_COLOR_SCHEME"))) {
	case "light":
		isDark.Store(false)
		pinned = true
	case "dark":
		pinned = true
	}
	dark.Sel = Mix(dark.Bg, dark.Accent, 0.15)
	light.Sel = Mix(light.Bg, light.Accent, 0.10)
}

func SetBackground(c color.Color, isDarkBg bool) {
	mu.Lock()
	defer mu.Unlock()
	if !pinned {
		isDark.Store(isDarkBg)
	}
	p := &light
	if isDark.Load() {
		p = &dark
	}
	if c != nil && isDark.Load() == isDarkBg {
		p.Bg = c
		amount := 0.10
		if isDark.Load() {
			amount = 0.15
		}
		p.Sel = Mix(p.Bg, p.Accent, amount)
	}
	version.Add(1)
}

func Dark() bool      { return isDark.Load() }
func Version() uint64 { return version.Load() }

func P() *Palette {
	mu.RLock()
	defer mu.RUnlock()
	if isDark.Load() {
		return &dark
	}
	return &light
}

func Mix(a, b color.Color, t float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	f := func(x, y uint32) uint8 { return uint8((float64(x>>8)*(1-t) + float64(y>>8)*t) + 0.5) }
	return color.RGBA{f(ar, br), f(ag, bg), f(ab, bb), 255}
}

func SweepAt(i, n int) color.Color {
	s := P().Sweep
	f := 0.5
	if n > 1 {
		f = float64(i) / float64(n-1)
	}
	if f <= 0.5 {
		return Mix(s[0], s[1], f*2)
	}
	return Mix(s[1], s[2], (f-0.5)*2)
}

func Fg(c color.Color) lipgloss.Style        { return lipgloss.NewStyle().Foreground(c) }
func Paint(s string, c color.Color) string   { return Fg(c).Render(s) }
func Bold(s string, c color.Color) string    { return Fg(c).Bold(true).Render(s) }
func On(s string, fg, bg color.Color) string { return Fg(fg).Background(bg).Render(s) }
