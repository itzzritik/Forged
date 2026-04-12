package importers

import "strings"

type ImportedKey struct {
	Name       string
	PrivateKey string
	PublicKey  string
}

func SanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, name)
	if name == "" {
		name = "imported"
	}
	return name
}
