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

type WorkerMessage = { type: "hash"; masterPasswordHash: Uint8Array } | { type: "key"; cryptoKey: CryptoKey } | { type: "error"; error: string };

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

export function derivePasswordHash(password: string, kdfParams: KDFParams): Promise<Uint8Array> {
	const worker = getWorker();
	return new Promise((resolve, reject) => {
		worker.onmessage = (e: MessageEvent<WorkerMessage>) => {
			if (e.data.type === "hash") {
				resolve(e.data.masterPasswordHash);
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

export async function encryptBlob(symmetricKey: CryptoKey, rawJson: string): Promise<Uint8Array> {
	const nonce = crypto.getRandomValues(new Uint8Array(12));
	const encoded = new TextEncoder().encode(rawJson);

	const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, symmetricKey, encoded));

	const result = new Uint8Array(nonce.length + ciphertext.length);
	result.set(nonce, 0);
	result.set(ciphertext, nonce.length);
	return result;
}
