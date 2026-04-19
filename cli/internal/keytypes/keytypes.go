package keytypes

import "strings"

type ID string

const (
	ED25519   ID = "ed25519"
	RSA       ID = "rsa"
	ECDSA     ID = "ecdsa"
	DSA       ID = "dsa"
	ED25519SK ID = "ed25519-sk"
	ECDSASK   ID = "ecdsa-sk"
)

var (
	all = []ID{
		ED25519,
		RSA,
		ECDSA,
		DSA,
		ED25519SK,
		ECDSASK,
	}
	aliases = map[string]ID{
		"ed25519":                            ED25519,
		"ssh-ed25519":                        ED25519,
		"rsa":                                RSA,
		"ssh-rsa":                            RSA,
		"rsa-sha2-256":                       RSA,
		"rsa-sha2-512":                       RSA,
		"ecdsa":                              ECDSA,
		"ecdsa-sha2-nistp256":                ECDSA,
		"ecdsa-sha2-nistp384":                ECDSA,
		"ecdsa-sha2-nistp521":                ECDSA,
		"dsa":                                DSA,
		"ssh-dss":                            DSA,
		"ed25519-sk":                         ED25519SK,
		"sk-ssh-ed25519@openssh.com":         ED25519SK,
		"ecdsa-sk":                           ECDSASK,
		"sk-ecdsa-sha2-nistp256@openssh.com": ECDSASK,
	}
)

func All() []ID {
	return append([]ID(nil), all...)
}

func Normalize(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}
	if id, ok := aliases[normalized]; ok {
		return string(id)
	}
	return normalized
}

func FromSSHPublicKeyType(raw string) string {
	return Normalize(raw)
}
