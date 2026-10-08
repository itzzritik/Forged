package core

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type Action struct {
	Key, Label, Icon string
	Danger           bool
}

type Screen interface {
	Update(tea.Msg, *State) (Screen, tea.Cmd)
	View(st *State, w, h int) string
	Actions(st *State) []Action
}

type Capturer interface{ Capturing() bool }

var namedKeys = map[string]rune{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEscape,
	"space":     tea.KeySpace,
	"tab":       tea.KeyTab,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"backspace": tea.KeyBackspace,
}

func Press(k string) tea.KeyPressMsg {
	if code, ok := namedKeys[k]; ok {
		if code == tea.KeySpace {
			return tea.KeyPressMsg{Code: code, Text: " "}
		}
		return tea.KeyPressMsg{Code: code}
	}
	if utf8.RuneCountInString(k) != 1 {
		return tea.KeyPressMsg{}
	}
	r, _ := utf8.DecodeRuneInString(k)
	return tea.KeyPressMsg{Code: r, Text: k}
}

type Sizer interface {
	Size(st *State, maxW, maxH int) (w, h int)
}
