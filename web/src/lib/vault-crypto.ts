export interface KDFParams {
	memory: number;
	parallelism: number;
	salt: string; // base64
	time: number;
}

export interface VaultKeyMetadata {
	comment: string;
	createdAt: string;
	fingerprint: string;
	gitSigning: boolean;
	hostRules: Array<{ match: string; type: string }>;
	id: string;
	lastUsedAt?: string;
	name: string;
	publicKey: string;
	type: string;
	updatedAt: string;
	version?: number;
}

export interface VaultKeyDetails extends VaultKeyMetadata {
	encryptedCipherKey: string;
	encryptedPrivateKey: string;
}

export interface VaultData {
	keyGeneration: number;
	keys: VaultKeyMetadata[];
	metadata: { createdAt: string; deviceId: string; deviceName: string };
	raw: string;
}

type WorkerMessage = { type: "derived" } | { type: "key"; cryptoKey: CryptoKey } | { type: "rekeyed"; newProtectedKey: string } | { type: "error"; error: string };

let activeWorker: Worker | null = null;

function getWorker(): Worker {
	if (!activeWorker) {
		activeWorker = new Worker(new URL("./vault-crypto-worker.ts", import.meta.url));
	}
	return activeWorker;
}

function terminateWorker() {
	activeWorker?.terminate();
	activeWorker = null;
}

export function deriveStretchedKey(password: string, kdfParams: KDFParams): Promise<void> {
	const worker = getWorker();
	return new Promise((resolve, reject) => {
		worker.onmessage = (e: MessageEvent<WorkerMessage>) => {
			if (e.data.type === "derived") {
				resolve();
			} else if (e.data.type === "error") {
				terminateWorker();
				reject(new Error(e.data.error));
			}
		};
		worker.onerror = (err) => {
			terminateWorker();
			reject(new Error(`Worker error: ${err.message}`));
		};
		worker.postMessage({ type: "derive", password, ...kdfParams });
	});
}

export function decryptProtectedKey(protectedSymmetricKey: string): Promise<CryptoKey> {
	const worker = getWorker();
	return new Promise((resolve, reject) => {
		worker.onmessage = (e: MessageEvent<WorkerMessage>) => {
			terminateWorker();
			if (e.data.type === "key") {
				resolve(e.data.cryptoKey);
			} else if (e.data.type === "error") {
				reject(new Error(e.data.error));
			}
		};
		worker.onerror = (err) => {
			terminateWorker();
			reject(new Error(`Worker error: ${err.message}`));
		};
		worker.postMessage({ type: "decrypt", protectedSymmetricKey });
	});
}

export function cancelDerivation() {
	terminateWorker();
}

export function generateKDFParams(): KDFParams {
	const salt = crypto.getRandomValues(new Uint8Array(32));
	return {
		salt: btoa(String.fromCharCode(...salt)),
		time: 3,
		memory: 65_536,
		parallelism: 4,
	};
}

export function rekeyProtectedKey(oldPassword: string, oldKdfParams: KDFParams, oldProtectedKey: string, newPassword: string, newKdfParams: KDFParams): Promise<string> {
	const worker = getWorker();
	return new Promise((resolve, reject) => {
		worker.onmessage = (e: MessageEvent<WorkerMessage>) => {
			terminateWorker();
			if (e.data.type === "rekeyed") resolve(e.data.newProtectedKey);
			else if (e.data.type === "error") reject(new Error(e.data.error));
		};
		worker.onerror = (err) => {
			terminateWorker();
			reject(new Error(`Worker error: ${err.message}`));
		};
		worker.postMessage({
			type: "rekey",
			oldPassword,
			oldSalt: oldKdfParams.salt,
			oldTime: oldKdfParams.time,
			oldMemory: oldKdfParams.memory,
			oldParallelism: oldKdfParams.parallelism,
			oldProtectedKey,
			newPassword,
			newSalt: newKdfParams.salt,
			newTime: newKdfParams.time,
			newMemory: newKdfParams.memory,
			newParallelism: newKdfParams.parallelism,
		});
	});
}

export async function decryptBlob(symmetricKey: CryptoKey, blob: Uint8Array): Promise<VaultData> {
	const nonce = blob.slice(0, 12);
	const ciphertext = blob.slice(12);

	const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, symmetricKey, ciphertext);

	const json = new TextDecoder().decode(plaintext);
	return vaultDataFromRaw(json);
}

// Mutation helpers that operate on the raw JSON (snake_case, preserves encrypted fields)

export function vaultDataFromRaw(rawJson: string): VaultData {
	return { ...decryptBlobSync(JSON.parse(rawJson) as Record<string, unknown>), raw: rawJson };
}

export function addKeyToVault(data: VaultData, snakeCaseKey: Record<string, unknown>, deviceId?: string): VaultData {
	const parsed = JSON.parse(data.raw) as Record<string, unknown>;
	const now = new Date().toISOString();
	const currentDeviceId = applyLocalDeviceMetadata(parsed, deviceId);
	const key = {
		...snakeCaseKey,
		created_at: snakeCaseKey.created_at ?? now,
		device_origin: snakeCaseKey.device_origin && snakeCaseKey.device_origin !== "web" ? snakeCaseKey.device_origin : currentDeviceId,
		updated_at: snakeCaseKey.updated_at ?? now,
		version: snakeCaseKey.version ?? 1,
	};
	parsed.keys = [...(((parsed.keys as Record<string, unknown>[]) ?? [])), key];
	bumpVersionVector(parsed, currentDeviceId);
	const raw = JSON.stringify(parsed);
	return vaultDataFromRaw(raw);
}

export function removeKeyFromVault(data: VaultData, keyId: string, deviceId?: string): VaultData {
	const parsed = JSON.parse(data.raw) as Record<string, unknown>;
	const keys = (parsed.keys as Record<string, unknown>[]) ?? [];
	const removed = keys.find((key) => key.id === keyId);
	parsed.keys = keys.filter((key) => key.id !== keyId);
	if (removed) {
		const currentDeviceId = applyLocalDeviceMetadata(parsed, deviceId);
		upsertTombstone(parsed, keyId, currentDeviceId, new Date().toISOString());
		bumpVersionVector(parsed, currentDeviceId);
	}
	const raw = JSON.stringify(parsed);
	return vaultDataFromRaw(raw);
}

export function updateKeyInVault(data: VaultData, keyId: string, updates: Record<string, unknown>, deviceId?: string): VaultData {
	const parsed = JSON.parse(data.raw) as Record<string, unknown>;
	const keys = ((parsed.keys as Record<string, unknown>[]) ?? []).map((key) => {
		if (key.id !== keyId) {
			return key;
		}

		const currentDeviceId = firstNonEmpty(deviceId, ((parsed.metadata as Record<string, unknown> | undefined)?.device_id as string | undefined), "web");
		return {
			...key,
			...updates,
			device_origin: (key.device_origin as string | undefined) ?? currentDeviceId,
			updated_at: new Date().toISOString(),
			version: Number(key.version ?? 0) + 1,
		};
	});
	const changed = keys.some((key) => key.id === keyId);
	parsed.keys = keys;
	if (changed) {
		const currentDeviceId = applyLocalDeviceMetadata(parsed, deviceId);
		bumpVersionVector(parsed, currentDeviceId);
	}
	const raw = JSON.stringify(parsed);
	return vaultDataFromRaw(raw);
}

export function getVaultKeyDetails(data: VaultData, keyId: string): VaultKeyDetails | null {
	const parsed = JSON.parse(data.raw) as Record<string, unknown>;
	const rawKeys = (parsed.keys as Record<string, unknown>[]) ?? [];
	const rawKey = rawKeys.find((entry) => entry.id === keyId);
	if (!rawKey) return null;

	return {
		id: rawKey.id as string,
		name: rawKey.name as string,
		type: rawKey.type as string,
		publicKey: rawKey.public_key as string,
		fingerprint: rawKey.fingerprint as string,
		comment: (rawKey.comment as string) || "",
		createdAt: (rawKey.created_at as string) || "",
		updatedAt: (rawKey.updated_at as string) || "",
		lastUsedAt: (rawKey.last_used_at as string) || undefined,
		version: Number(rawKey.version ?? 0) || undefined,
		hostRules: (rawKey.host_rules as VaultKeyMetadata["hostRules"]) || [],
		gitSigning: Boolean(rawKey.git_signing),
		encryptedCipherKey: (rawKey.encrypted_cipher_key as string) || "",
		encryptedPrivateKey: (rawKey.encrypted_private_key as string) || "",
	};
}

function decryptBlobSync(parsed: Record<string, unknown>): Omit<VaultData, "raw"> {
	const keys: VaultKeyMetadata[] = ((parsed.keys as Record<string, unknown>[]) || []).map((k) => ({
		id: k.id as string,
		name: k.name as string,
		type: k.type as string,
		publicKey: k.public_key as string,
		fingerprint: k.fingerprint as string,
		comment: (k.comment as string) || "",
		createdAt: k.created_at as string,
		updatedAt: k.updated_at as string,
		lastUsedAt: (k.last_used_at as string) || undefined,
		hostRules: (k.host_rules as VaultKeyMetadata["hostRules"]) || [],
		gitSigning: Boolean(k.git_signing),
		version: Number(k.version ?? 0) || undefined,
	}));
	const meta = parsed.metadata as Record<string, unknown> | undefined;
	return {
		keys,
		metadata: {
			createdAt: (meta?.created_at as string) || "",
			deviceId: (meta?.device_id as string) || "",
			deviceName: (meta?.device_name as string) || "",
		},
		keyGeneration: (parsed.key_generation as number) || 1,
	};
}

function applyLocalDeviceMetadata(parsed: Record<string, unknown>, deviceId?: string): string {
	const metadata = (parsed.metadata as Record<string, unknown> | undefined) ?? {};
	const currentDeviceId = firstNonEmpty(deviceId, metadata.device_id as string | undefined, "web");
	parsed.metadata = {
		...metadata,
		device_id: currentDeviceId,
		device_name: (metadata.device_name as string | undefined) ?? "Browser",
	};
	return currentDeviceId;
}

function bumpVersionVector(parsed: Record<string, unknown>, deviceId: string) {
	const vector = { ...((parsed.version_vector as Record<string, number> | undefined) ?? {}) };
	vector[deviceId] = Number(vector[deviceId] ?? 0) + 1;
	parsed.version_vector = vector;
}

function upsertTombstone(parsed: Record<string, unknown>, keyId: string, deviceId: string, deletedAt: string) {
	const tombstones = ((parsed.tombstones as Record<string, unknown>[]) ?? []).map((tombstone) => ({ ...tombstone }));
	const next = {
		key_id: keyId,
		deleted_at: deletedAt,
		deleted_by_device: deviceId,
	};
	const index = tombstones.findIndex((tombstone) => tombstone.key_id === keyId);
	if (index >= 0) {
		const existing = tombstones[index];
		if (Date.parse(deletedAt) > Date.parse(String(existing.deleted_at ?? ""))) {
			tombstones[index] = next;
		}
	} else {
		tombstones.push(next);
	}
	parsed.tombstones = tombstones;
}

function firstNonEmpty(...values: Array<string | undefined>): string {
	for (const value of values) {
		if (value) {
			return value;
		}
	}
	return "";
}

export async function encryptBlob(symmetricKey: CryptoKey, rawJson: string): Promise<Uint8Array> {
	const nonce = crypto.getRandomValues(new Uint8Array(12));
	const encoded = new TextEncoder().encode(rawJson);

	const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, symmetricKey, encoded));

	const result = new Uint8Array(nonce.length + ciphertext.length);
	result.set(nonce, 0);
	result.set(ciphertext, nonce.length);
	return result;
}

export async function decryptItemKey(symmetricKey: CryptoKey, encryptedCipherKeyB64: string): Promise<CryptoKey> {
	const data = Uint8Array.from(atob(encryptedCipherKeyB64), (c) => c.charCodeAt(0));
	const nonce = data.slice(0, 12);
	const ciphertext = data.slice(12);

	const rawKey = await crypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, symmetricKey, ciphertext);

	return crypto.subtle.importKey("raw", rawKey, "AES-GCM", false, ["decrypt"]);
}

export async function decryptPrivateKey(cipherKey: CryptoKey, encryptedPrivateKeyB64: string): Promise<Uint8Array> {
	const data = Uint8Array.from(atob(encryptedPrivateKeyB64), (c) => c.charCodeAt(0));
	const nonce = data.slice(0, 12);
	const ciphertext = data.slice(12);

	const plaintext = await crypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, cipherKey, ciphertext);

	return new Uint8Array(plaintext);
}

export async function decryptVaultKeyPrivateKey(key: VaultKeyDetails, symmetricKey: CryptoKey): Promise<string> {
	if (!key.encryptedCipherKey || !key.encryptedPrivateKey) {
		throw new Error("Private key is unavailable for this item");
	}
	const cipherKey = await decryptItemKey(symmetricKey, key.encryptedCipherKey);
	const privateKeyBytes = await decryptPrivateKey(cipherKey, key.encryptedPrivateKey);
	return new TextDecoder().decode(privateKeyBytes);
}

export async function encryptNewItemKey(symmetricKey: CryptoKey): Promise<{ cipherKey: CryptoKey; encryptedCipherKeyB64: string }> {
	const cipherKey = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, true, ["encrypt", "decrypt"]);
	const rawKey = await crypto.subtle.exportKey("raw", cipherKey);
	const nonce = crypto.getRandomValues(new Uint8Array(12));
	const encrypted = await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, symmetricKey, rawKey);
	const combined = new Uint8Array(12 + encrypted.byteLength);
	combined.set(nonce);
	combined.set(new Uint8Array(encrypted), 12);
	return { cipherKey, encryptedCipherKeyB64: btoa(String.fromCharCode(...combined)) };
}

export async function encryptPrivateKey(cipherKey: CryptoKey, privateKeyBytes: Uint8Array): Promise<string> {
	const nonce = crypto.getRandomValues(new Uint8Array(12));
	const encrypted = await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, cipherKey, privateKeyBytes.buffer as ArrayBuffer);
	const combined = new Uint8Array(12 + encrypted.byteLength);
	combined.set(nonce);
	combined.set(new Uint8Array(encrypted), 12);
	return btoa(String.fromCharCode(...combined));
}
