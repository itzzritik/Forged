package theme

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ColorText    = lipgloss.AdaptiveColor{Light: "#18181b", Dark: "#fafafa"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#52525b", Dark: "#a1a1aa"}
	ColorSubtle  = lipgloss.AdaptiveColor{Light: "#6f6f78", Dark: "#8a8a94"}
	ColorBorder  = lipgloss.AdaptiveColor{Light: "#8b8b95", Dark: "#666670"}
	ColorAccent  = lipgloss.AdaptiveColor{Light: "#c2410c", Dark: "#f97316"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#22c55e"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#a16207", Dark: "#f59e0b"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#ef4444"}
)

func init() {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FORGED_COLOR_SCHEME"))) {
	case "light":
		lipgloss.SetHasDarkBackground(false)
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	}
}

const (
	ShellMinContentWidth = 64
	ShellMaxContentWidth = 108
	ShellHorizontalInset = 6
	ShellLeftInset       = 2
	HeroMaxWidth         = 92
)
