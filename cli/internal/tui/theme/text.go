package theme

import (
	"strings"
	"unicode"
)

func SanitizeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, value)
}
