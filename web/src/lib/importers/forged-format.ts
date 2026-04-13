import { DEFAULT_IMPORTED_NAME, normalizeImportedName } from "./name";
import type { ImportedKey } from "./types";

export function parseForged(data: string): ImportedKey[] {
	const parsed = JSON.parse(data);
	if (parsed.format !== "forged-export") throw new Error("Not a Forged export file");

	const keys: ImportedKey[] = [];
	for (const item of parsed.items || []) {
		if (item.type !== "ssh_key" || !item.ssh_key?.private_key) continue;
		keys.push({
			name: normalizeImportedName(item.name || DEFAULT_IMPORTED_NAME),
			privateKey: item.ssh_key.private_key,
			publicKey: item.ssh_key.public_key,
			fingerprint: item.ssh_key.fingerprint,
		});
	}
	return keys;
}
