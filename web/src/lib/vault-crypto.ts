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
	name: string;
	publicKey: string;
	type: string;
	updatedAt: string;
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
	const parsed = JSON.parse(json);

	const keys: VaultKeyMetadata[] = (parsed.keys || []).map((k: Record<string, unknown>) => ({
		id: k.id,
		name: k.name,
		type: k.type,
		publicKey: k.public_key,
		fingerprint: k.fingerprint,
		comment: k.comment,
		createdAt: k.created_at,
		updatedAt: k.updated_at,
		hostRules: k.host_rules || [],
		gitSigning: k.git_signing,
	}));

	return {
		keys,
		metadata: {
			createdAt: parsed.metadata?.created_at || "",
			deviceId: parsed.metadata?.device_id || "",
			deviceName: parsed.metadata?.device_name || "",
		},
		keyGeneration: parsed.key_generation || 1,
		raw: json,
	};
}

// Mutation helpers that operate on the raw JSON (snake_case, preserves encrypted fields)

export function addKeyToVault(data: VaultData, snakeCaseKey: Record<string, unknown>): VaultData {
	const parsed = JSON.parse(data.raw);
	parsed.keys = [...(parsed.keys || []), snakeCaseKey];
	const raw = JSON.stringify(parsed);
	return { ...decryptBlobSync(parsed), raw };
}

export function removeKeyFromVault(data: VaultData, keyId: string): VaultData {
	const parsed = JSON.parse(data.raw);
	parsed.keys = (parsed.keys || []).filter((k: Record<string, unknown>) => k.id !== keyId);
	const raw = JSON.stringify(parsed);
	return { ...decryptBlobSync(parsed), raw };
}

export function updateKeyInVault(data: VaultData, keyId: string, updates: Record<string, unknown>): VaultData {
	const parsed = JSON.parse(data.raw);
	parsed.keys = (parsed.keys || []).map((k: Record<string, unknown>) => (k.id === keyId ? { ...k, ...updates, updated_at: new Date().toISOString() } : k));
	const raw = JSON.stringify(parsed);
	return { ...decryptBlobSync(parsed), raw };
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
		hostRules: (k.host_rules as VaultKeyMetadata["hostRules"]) || [],
		gitSigning: Boolean(k.git_signing),
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
