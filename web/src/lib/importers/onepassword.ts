import { unzipSync } from "fflate";
import type { ImportedKey } from "./types";

const SANITIZE_RE = /[^a-z0-9_-]/g;

export function parse1Password(data: Uint8Array): ImportedKey[] {
	const files = unzipSync(data);
	const exportDataBytes = files["export.data"];
	if (!exportDataBytes) throw new Error("export.data not found in 1pux archive");

	const exportData = JSON.parse(new TextDecoder().decode(exportDataBytes));
	const keys: ImportedKey[] = [];

	for (const account of exportData.accounts || []) {
		for (const vault of account.vaults || []) {
			for (const item of vault.items || []) {
				if (item.categoryUuid !== "114") continue;
				const privKey = extractSSHKey(item);
				if (!privKey) continue;
				keys.push({
					name: sanitizeName(item.overview?.title || "imported"),
					privateKey: privKey,
				});
			}
		}
	}
	return keys;
}

function extractSSHKey(item: Record<string, unknown>): string | null {
	const details = item.details as Record<string, unknown> | undefined;
	if (!details) return null;

	for (const section of (details.sections as Record<string, unknown>[]) || []) {
		for (const field of (section.fields as Record<string, unknown>[]) || []) {
			const value = field.value;
			if (typeof value === "object" && value !== null) {
				const sshKey = (value as Record<string, Record<string, string>>).sshKey;
				if (sshKey?.privateKey) return sshKey.privateKey;
			}
			if (typeof value === "string" && value.includes("PRIVATE KEY")) return value;
		}
	}
	return null;
}

function sanitizeName(name: string): string {
	return name.toLowerCase().replace(/\s+/g, "-").replace(SANITIZE_RE, "") || "imported";
}
