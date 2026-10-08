package ui

import (
	"os"
	"strings"
)

type Glyphs struct {
	Brand, Dot, Ring, Bar, Caret, Next, Prev, Check, Cross, Warn, Box, BoxOn string
	Heavy, H, V, TL, TR, BL, BR, Down, UpDown, Ellipsis, Mask, Spark         string
	Spinner                                                                  []string
}

var G = pick()

var asciiMode bool

func ASCII() bool { return asciiMode }

func pick() Glyphs {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FORGED_ASCII")))
	asciiMode = v == "1" || v == "true" || !terminalSupportsUnicode()
	if asciiMode {
		return Glyphs{Brand: "*", Dot: "*", Ring: "o", Bar: "|", Caret: "v", Next: ">", Prev: "<", Check: "+", Cross: "x",
			Warn: "!", Box: "[ ]", BoxOn: "[x]", Heavy: "=", H: "-", V: "|", TL: "+", TR: "+", BL: "+", BR: "+",
			Down: "v", UpDown: "^v", Ellipsis: "~", Mask: "*", Spark: ".", Spinner: []string{"|", "/", "-", "\\"}}
	}
	return Glyphs{Brand: "◆", Dot: "●", Ring: "○", Bar: "▎", Caret: "▾", Next: "›", Prev: "‹", Check: "✓", Cross: "✕",
		Warn: "!", Box: "□", BoxOn: "■", Heavy: "━", H: "─", V: "│", TL: "╭", TR: "╮", BL: "╰", BR: "╯",
		Down: "↓", UpDown: "↑↓", Ellipsis: "…", Mask: "•", Spark: "·",
		Spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}}
}
