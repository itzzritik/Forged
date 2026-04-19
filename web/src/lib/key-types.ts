export const KEY_TYPE_ED25519 = "ed25519";
export const KEY_TYPE_RSA = "rsa";
export const KEY_TYPE_ECDSA = "ecdsa";
export const KEY_TYPE_DSA = "dsa";
export const KEY_TYPE_ED25519_SK = "ed25519-sk";
export const KEY_TYPE_ECDSA_SK = "ecdsa-sk";

export type KeyTypeId =
	| typeof KEY_TYPE_ED25519
	| typeof KEY_TYPE_RSA
	| typeof KEY_TYPE_ECDSA
	| typeof KEY_TYPE_DSA
	| typeof KEY_TYPE_ED25519_SK
	| typeof KEY_TYPE_ECDSA_SK;

export const ALL_KEY_TYPE_IDS: KeyTypeId[] = [KEY_TYPE_ED25519, KEY_TYPE_RSA, KEY_TYPE_ECDSA, KEY_TYPE_DSA, KEY_TYPE_ED25519_SK, KEY_TYPE_ECDSA_SK];

const KEY_TYPE_ALIASES: Record<string, KeyTypeId> = {
	ed25519: KEY_TYPE_ED25519,
	"ssh-ed25519": KEY_TYPE_ED25519,
	rsa: KEY_TYPE_RSA,
	"ssh-rsa": KEY_TYPE_RSA,
	"rsa-sha2-256": KEY_TYPE_RSA,
	"rsa-sha2-512": KEY_TYPE_RSA,
	ecdsa: KEY_TYPE_ECDSA,
	"ecdsa-sha2-nistp256": KEY_TYPE_ECDSA,
	"ecdsa-sha2-nistp384": KEY_TYPE_ECDSA,
	"ecdsa-sha2-nistp521": KEY_TYPE_ECDSA,
	dsa: KEY_TYPE_DSA,
	"ssh-dss": KEY_TYPE_DSA,
	"ed25519-sk": KEY_TYPE_ED25519_SK,
	"sk-ssh-ed25519@openssh.com": KEY_TYPE_ED25519_SK,
	"ecdsa-sk": KEY_TYPE_ECDSA_SK,
	"sk-ecdsa-sha2-nistp256@openssh.com": KEY_TYPE_ECDSA_SK,
};

export function normalizeKeyTypeId(value: string | null | undefined): string {
	const normalized = (value ?? "").trim().toLowerCase();
	if (!normalized) {
		return "";
	}
	return KEY_TYPE_ALIASES[normalized] ?? normalized;
}
