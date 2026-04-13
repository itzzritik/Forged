package importers

import (
	"fmt"
	"strings"
)

type ImportedKey struct {
	Name       string
	PrivateKey string
	PublicKey  string
}

const DefaultImportedName = "Imported"

func SanitizeName(name string) string {
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
