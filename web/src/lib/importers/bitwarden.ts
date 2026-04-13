import { DEFAULT_IMPORTED_NAME, normalizeImportedName } from "./name";
import type { ImportedKey } from "./types";

export function parseBitwarden(data: string): ImportedKey[] {
	const parsed = JSON.parse(data);
	if (parsed.encrypted) throw new Error("Encrypted Bitwarden exports are not supported. Export as unencrypted JSON.");

	const keys: ImportedKey[] = [];
	for (const item of parsed.items || []) {
		if (item.type !== 5 || !item.sshKey?.privateKey) continue;
		keys.push({
			name: normalizeImportedName(item.name || DEFAULT_IMPORTED_NAME),
			privateKey: item.sshKey.privateKey,
			publicKey: item.sshKey.publicKey,
			fingerprint: item.sshKey.keyFingerprint,
		});
	}
	return keys;
}
