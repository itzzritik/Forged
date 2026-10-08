package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
}

func Width(s string) int { return ansi.StringWidth(s) }

func Repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

func Trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, G.Ellipsis)
}

func MidTrunc(s string, n int) string {
	if Width(s) <= n {
		return s
	}
	if n < 5 {
		return Trunc(s, n)
	}
	head := n / 2
	tail := (n - 1) / 2
	return ansi.Truncate(s, head, "") + G.Ellipsis + ansi.TruncateLeft(s, Width(s)-tail, "")
}

func Pad(s string, n int) string { return s + Repeat(" ", n-Width(s)) }

func Wrap(s string, n int) []string {
	if n <= 0 {
		return nil
	}
	return strings.Split(ansi.Hardwrap(ansi.Wordwrap(s, n, " -"), n, true), "\n")
}

func Join(lines []string) string { return strings.Join(lines, "\n") }
