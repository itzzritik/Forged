import { parseBitwarden } from "@/lib/importers/bitwarden";
import { parseForged } from "@/lib/importers/forged-format";
import { normalizeImportedName } from "@/lib/importers/name";
import { parse1Password } from "@/lib/importers/onepassword";
import type { ImportedKey } from "@/lib/importers/types";
import { type ParsedSSHKey, parseSSHKeyFile } from "@/lib/ssh-key-parser";
import { computeFingerprintFromBlob, formatSSHPublicKeyFromBlob } from "@/lib/ssh-key-utils";
import { addKeyToVault, encryptNewItemKey, encryptPrivateKey, type VaultData } from "@/lib/vault-crypto";

export type Source = "ssh" | "forged" | "1password" | "bitwarden";

export const SOURCES: { accept: string; hint: string; id: Source; title: string }[] = [
	{ accept: "*", hint: "One unencrypted OpenSSH, PKCS#8 or PEM private key, like id_ed25519.", id: "ssh", title: "SSH key file" },
	{ accept: ".json", hint: "A .json file from an earlier Forged export.", id: "forged", title: "Forged export" },
	{ accept: ".1pux,.csv,text/csv", hint: "A .1pux export works best. A .csv works when rows hold private keys.", id: "1password", title: "1Password" },
	{ accept: ".json", hint: "An unencrypted .json vault export from Settings, Export vault.", id: "bitwarden", title: "Bitwarden" },
];

export interface Candidate {
	duplicate: boolean;
	fingerprint: string;
	name: string;
	parsed: ParsedSSHKey;
	repeats: number;
}

const EXTENSION = /\.(pem|key|pub)$/i;
const ID_PREFIX = /^id_/;
const UNDERSCORES = /_/g;

async function readEntries(source: Source, file: File): Promise<ImportedKey[]> {
	if (source === "1password") return parse1Password(new Uint8Array(await file.arrayBuffer()));
	if (source === "bitwarden") return parseBitwarden(await file.text());
	if (source === "forged") return parseForged(await file.text());
	return [{ name: "", privateKey: await file.text() }];
}

export async function parseImport(source: Source, file: File, existing: Set<string>) {
	const fallbackName = normalizeImportedName(file.name.replace(EXTENSION, "").replace(ID_PREFIX, "").replace(UNDERSCORES, " "));
	const entries = await readEntries(source, file);
	const settled = await Promise.allSettled(
		entries.map(async (entry) => {
			const parsed = await parseSSHKeyFile(entry.privateKey);
			return { fingerprint: await computeFingerprintFromBlob(parsed.publicKeyBlob), name: entry.name || parsed.comment || fallbackName, parsed };
		})
	);
	const byFingerprint = new Map<string, Candidate>();
	let failed = 0;
	let firstError: unknown = null;
	for (const result of settled) {
		if (result.status === "rejected") {
			failed++;
			firstError ??= result.reason;
			continue;
		}
		const seen = byFingerprint.get(result.value.fingerprint);
		if (seen) seen.repeats++;
		else byFingerprint.set(result.value.fingerprint, { ...result.value, duplicate: existing.has(result.value.fingerprint), repeats: 0 });
	}
	const candidates = [...byFingerprint.values()].sort((a, b) => Number(a.duplicate) - Number(b.duplicate));
	if (candidates.length === 0 && firstError) throw firstError;
	return { candidates, failed };
}

export function importError(error: unknown): string {
	const message = error instanceof Error ? error.message : "Couldn't read this file.";
	if (message === "PASSPHRASE_PROTECTED") return "This key has a passphrase. Remove it with ssh-keygen -p -f <file>, then import again.";
	if (message === "UNKNOWN_FORMAT") return "Forged can't read this format. Use an unencrypted OpenSSH or PEM private key.";
	return message;
}

export async function addImported(vault: VaultData, symmetricKey: CryptoKey, picks: Candidate[], deviceId: string, onProgress: (done: number) => void) {
	let next = vault;
	const ids: string[] = [];
	for (const [index, pick] of picks.entries()) {
		const { cipherKey, encryptedCipherKeyB64 } = await encryptNewItemKey(symmetricKey);
		const encryptedPrivateKey = await encryptPrivateKey(cipherKey, pick.parsed.privateKeyBytes);
		const id = crypto.randomUUID();
		const now = new Date().toISOString();
		next = addKeyToVault(
			next,
			{
				comment: pick.parsed.comment,
				created_at: now,
				device_origin: "web",
				encrypted_cipher_key: encryptedCipherKeyB64,
				encrypted_private_key: encryptedPrivateKey,
				fingerprint: pick.fingerprint,
				git_signing: false,
				id,
				name: pick.name,
				public_key: formatSSHPublicKeyFromBlob(pick.parsed.publicKeyBlob, pick.name),
				tags: [],
				type: pick.parsed.type,
				updated_at: now,
				version: 1,
			},
			deviceId
		);
		ids.push(id);
		onProgress(index + 1);
	}
	return { ids, next };
}
