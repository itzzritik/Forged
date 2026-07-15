package importers

import (
	"fmt"
	"strings"
	"unicode"
)

type ImportedKey struct {
	Name       string
	PrivateKey string
	PublicKey  string
}

const DefaultImportedName = "Imported"

func stripUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

func SanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if name == "" {
		return DefaultImportedName
	}
	return name
}

func FallbackImportedName(ordinal int) string {
	if ordinal <= 0 {
		return DefaultImportedName
	}
	return fmt.Sprintf("%s %d", DefaultImportedName, ordinal)
}
