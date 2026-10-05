package theme

import (
	"image/color"
	"os"
	"strings"
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

var (
	ColorText    = adaptive("#18181b", "#fafafa")
	ColorMuted   = adaptive("#52525b", "#a1a1aa")
	ColorSubtle  = adaptive("#6f6f78", "#8a8a94")
	ColorBorder  = adaptive("#8b8b95", "#666670")
	ColorAccent  = adaptive("#c2410c", "#f97316")
	ColorSuccess = adaptive("#15803d", "#22c55e")
	ColorWarning = adaptive("#a16207", "#f59e0b")
	ColorDanger  = adaptive("#b91c1c", "#ef4444")
)

var (
	darkBackground atomic.Bool
	schemePinned   bool
)

// adaptiveColor resolves at render time. lipgloss/v2/compat does the same but probes the
// terminal at package init, which would hit every forged process, not just the TUI.
type adaptiveColor struct {
	light, dark color.Color
}

func adaptive(light, dark string) color.Color {
	return adaptiveColor{light: lipgloss.Color(light), dark: lipgloss.Color(dark)}
}

func (c adaptiveColor) RGBA() (r, g, b, a uint32) {
	if darkBackground.Load() {
		return c.dark.RGBA()
	}
	return c.light.RGBA()
}

// SetDarkBackground applies the background the terminal reported, unless
// FORGED_COLOR_SCHEME pins the scheme.
func SetDarkBackground(dark bool) {
	if !schemePinned {
		darkBackground.Store(dark)
	}
}

func init() {
	// Dark until the terminal answers, matching v1's fallback when detection fails.
	darkBackground.Store(true)
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FORGED_COLOR_SCHEME"))) {
	case "light":
		darkBackground.Store(false)
		schemePinned = true
	case "dark":
		schemePinned = true
	}
}

const (
	ShellMinContentWidth = 64
	ShellMaxContentWidth = 108
	ShellHorizontalInset = 6
	ShellLeftInset       = 2
	HeroMaxWidth         = 92
)
