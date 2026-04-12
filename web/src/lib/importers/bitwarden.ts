import type { ImportedKey } from "./types";

const SANITIZE_RE = /[^a-z0-9_-]/g;

export function parseBitwarden(data: string): ImportedKey[] {
	const parsed = JSON.parse(data);
	if (parsed.encrypted) throw new Error("Encrypted Bitwarden exports are not supported. Export as unencrypted JSON.");

	const keys: ImportedKey[] = [];
	for (const item of parsed.items || []) {
		if (item.type !== 5 || !item.sshKey?.privateKey) continue;
		keys.push({
			name: sanitizeName(item.name || "imported"),
			privateKey: item.sshKey.privateKey,
			publicKey: item.sshKey.publicKey,
			fingerprint: item.sshKey.keyFingerprint,
		});
	}
	return keys;
}

function sanitizeName(name: string): string {
	return name.toLowerCase().replace(/\s+/g, "-").replace(SANITIZE_RE, "") || "imported";
}
