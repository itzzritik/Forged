package theme

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

type GlyphSet struct {
	Unicode       bool
	SpinnerFrames []string
	Check         string
	Cross         string
	Sparkle       string
	Pending       string
	Selection     string
	Prompt        string
	Breadcrumb    string
	Bullet        string
	Separator     string
	Horizontal    string
	Vertical      string
	TopLeft       string
	TopRight      string
	BottomLeft    string
	BottomRight   string
	Up            string
	Down          string
	UpDown        string
	LeftRight     string
	Mask          string
	Ellipsis      string
	Empty         string
	Checked       string
	Unchecked     string
}

const unicodeEllipsis = "…"

var Glyphs = selectGlyphs()

func selectGlyphs() GlyphSet {
	if forceASCII() || !terminalSupportsUnicode() {
		return GlyphSet{
			SpinnerFrames: []string{"|", "/", "-", "\\"},
			Check:         "+",
			Cross:         "x",
			Sparkle:       "*",
			Pending:       "...",
			Selection:     ">",
			Prompt:        ">",
			Breadcrumb:    ">",
			Bullet:        "-",
			Separator:     "|",
			Horizontal:    "-",
			Vertical:      "|",
			TopLeft:       "+",
			TopRight:      "+",
			BottomLeft:    "+",
			BottomRight:   "+",
			Up:            "^",
			Down:          "v",
			UpDown:        "^/v",
			LeftRight:     "</>",
			Mask:          "*",
			Ellipsis:      "~",
			Empty:         "-",
			Checked:       "#",
			Unchecked:     ".",
		}
	}

	return GlyphSet{
		Unicode:       true,
		SpinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Check:         "✓",
		Cross:         "✕",
		Sparkle:       "✦",
		Pending:       unicodeEllipsis,
		Selection:     "▸",
		Prompt:        "❯",
		Breadcrumb:    "❱",
		Bullet:        "·",
		Separator:     "·",
		Horizontal:    "─",
		Vertical:      "│",
		TopLeft:       "┌",
		TopRight:      "┐",
		BottomLeft:    "└",
		BottomRight:   "┘",
		Up:            "↑",
		Down:          "↓",
		UpDown:        "↑/↓",
		LeftRight:     "←/→",
		Mask:          "•",
		Ellipsis:      unicodeEllipsis,
		Empty:         "—",
		Checked:       "■",
		Unchecked:     "□",
	}
}

func forceASCII() bool {
	value := strings.TrimSpace(os.Getenv("FORGED_ASCII"))
	return value == "1" || strings.EqualFold(value, "true")
}

func AdaptTextInputPlaceholder(view string, value string) string {
	if value != "" {
		view = strings.ReplaceAll(view, value, SanitizeText(value))
	}
	if Glyphs.Unicode || value != "" {
		return view
	}
	return strings.ReplaceAll(view, unicodeEllipsis, Glyphs.Ellipsis)
}

func NormalBorder() lipgloss.Border {
	if Glyphs.Unicode {
		return lipgloss.NormalBorder()
	}
	return lipgloss.ASCIIBorder()
}

func RoundedBorder() lipgloss.Border {
	if Glyphs.Unicode {
		return lipgloss.RoundedBorder()
	}
	return lipgloss.ASCIIBorder()
}
