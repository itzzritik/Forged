package ui

import (
	"os"
	"strings"
)

type Glyphs struct {
	Dot, Ring, Bar, Caret, Next, Prev, Check, Cross, Warn, Box, BoxOn string
	Heavy, H, V, TL, TR, BL, BR, Down, UpDown, Ellipsis, Mask, Spark  string
	Spinner                                                           []string
	Icon                                                              Icons
}

type Icons struct {
	SSH, Sign, Routes, Sync, Login, Clock, Secret, Logout, New, Import string
	Export, Lock, Copy, Print, Rename, Delete, Health, Refresh, Quit   string
	Home, File, Paste, Details, Search                                 string
}

var G = pick()

var asciiMode bool

func ASCII() bool { return asciiMode }

func pick() Glyphs {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FORGED_ASCII")))
	asciiMode = v == "1" || v == "true" || !terminalSupportsUnicode()
	if asciiMode {
		return Glyphs{Dot: "*", Ring: "o", Bar: "|", Caret: "v", Next: ">", Prev: "<", Check: "+", Cross: "x",
			Warn: "!", Box: "[ ]", BoxOn: "[x]", Heavy: "=", H: "-", V: "|", TL: "+", TR: "+", BL: "+", BR: "+",
			Down: "v", UpDown: "^v", Ellipsis: "~", Mask: "*", Spark: ".", Spinner: []string{"|", "/", "-", "\\"}}
	}
	return Glyphs{Dot: "●", Ring: "○", Bar: "▎", Caret: "▾", Next: "›", Prev: "‹", Check: "✓", Cross: "✕",
		Warn: "!", Box: "□", BoxOn: "■", Heavy: "━", H: "─", V: "│", TL: "╭", TR: "╮", BL: "╰", BR: "╯",
		Down: "↓", UpDown: "↑↓", Ellipsis: "…", Mask: "•", Spark: "·",
		Spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Icon: Icons{SSH: "⇄", Sign: "✎", Routes: "☍", Sync: "↻", Login: "⇥", Clock: "◷", Secret: "✱", Logout: "⎋", New: "⊕",
			Import: "↧", Export: "↥", Lock: "⊡", Copy: "❐", Print: "◍", Rename: "⎀", Delete: "⌫", Health: "✚", Refresh: "↺", Quit: "⊗",
			Home: "⌂", File: "⎕", Paste: "⇣", Details: "⌸", Search: "⌕"}}
}
