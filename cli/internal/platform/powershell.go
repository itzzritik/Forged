package platform

import (
	"encoding/base64"
	"unicode/utf16"
)

func UTF16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2*len(units))
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func PowerShellEncodedCommand(script string) string {
	return base64.StdEncoding.EncodeToString(UTF16LE(script))
}
