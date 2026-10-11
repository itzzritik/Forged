import { KEY_TYPE_ED25519, normalizeKeyTypeId } from "@/lib/key-types";
import { exportPrivateKeyToOpenSSH } from "@/lib/ssh-key-parser";
import { computeFingerprint, formatSSHPublicKey } from "@/lib/ssh-key-utils";
import {
	addKeyToVault,
	decryptItemKey,
	decryptPrivateKey,
	encryptNewItemKey,
	encryptPrivateKey,
	removeKeyFromVault,
	type VaultData,
	type VaultKeyMetadata,
} from "@/lib/vault-crypto";

export const NAME_LIMIT = 128;

export function validateName(name: string, keys: VaultKeyMetadata[], selfId?: string): string | null {
	if (!name) return "Give the key a name.";
	if (name.length > NAME_LIMIT) return `Keep the name under ${NAME_LIMIT} characters.`;
	const lower = name.toLowerCase();
	if (keys.some((key) => key.id !== selfId && key.name.toLowerCase() === lower)) return "Another key already has this name. Pick a different one.";
	return null;
}

export async function createKey(vault: VaultData, symmetricKey: CryptoKey, name: string, deviceId: string) {
	const pair = (await crypto.subtle.generateKey("Ed25519", true, ["sign", "verify"])) as CryptoKeyPair;
	const publicRaw = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
	const privateBytes = await exportPrivateKeyToOpenSSH(pair.privateKey, KEY_TYPE_ED25519);
	const { cipherKey, encryptedCipherKeyB64 } = await encryptNewItemKey(symmetricKey);
	const encryptedPrivateKey = await encryptPrivateKey(cipherKey, privateBytes);
	privateBytes.fill(0);
	const id = crypto.randomUUID();
	const now = new Date().toISOString();
	const next = addKeyToVault(
		vault,
		{
			comment: "",
			created_at: now,
			device_origin: "web",
			encrypted_cipher_key: encryptedCipherKeyB64,
			encrypted_private_key: encryptedPrivateKey,
			fingerprint: await computeFingerprint(publicRaw),
			git_signing: false,
			id,
			name,
			public_key: formatSSHPublicKey(publicRaw, name),
			tags: [],
			type: KEY_TYPE_ED25519,
			updated_at: now,
			version: 1,
		},
		deviceId
	);
	return { id, next };
}

export function deleteKeys(vault: VaultData, ids: string[], deviceId: string): VaultData {
	return ids.reduce((current, id) => removeKeyFromVault(current, id, deviceId), vault);
}

export const exportFileName = () => `forged-export-${new Date().toISOString().slice(0, 10)}.json`;

export async function exportKeys(raw: string, ids: ReadonlySet<string> | null, symmetricKey: CryptoKey, fileName: string) {
	const rawKeys = ((JSON.parse(raw) as { keys?: Record<string, unknown>[] }).keys ?? []).filter((key) => !ids || ids.has(String(key.id)));
	const decoder = new TextDecoder();
	const decrypted = await Promise.all(
		rawKeys.map(async (key) => {
			const wrapped = String(key.encrypted_cipher_key ?? "");
			const sealed = String(key.encrypted_private_key ?? "");
			if (!(wrapped && sealed)) return null;
			const privateBytes = await decryptPrivateKey(await decryptItemKey(symmetricKey, wrapped), sealed);
			const privateKey = decoder.decode(privateBytes);
			privateBytes.fill(0);
			return {
				created_at: key.created_at ?? "",
				name: key.name,
				ssh_key: {
					comment: key.comment ?? "",
					fingerprint: key.fingerprint ?? "",
					git_signing: Boolean(key.git_signing),
					key_type: normalizeKeyTypeId(String(key.type ?? "")),
					private_key: privateKey,
					public_key: key.public_key ?? "",
				},
				type: "ssh_key",
				updated_at: key.updated_at ?? "",
			};
		})
	);
	const items = decrypted.filter((item) => item !== null);
	const file = new Blob([JSON.stringify({ exported_at: new Date().toISOString(), format: "forged-export", items, version: 1 }, null, 2)], { type: "application/json" });
	const url = URL.createObjectURL(file);
	const link = document.createElement("a");
	link.href = url;
	link.download = fileName;
	link.click();
	URL.revokeObjectURL(url);
	return items.length;
}
