import argon2 from "argon2-browser/dist/argon2-bundled.min.js";

let stretchedMasterKey: Uint8Array | null = null;

self.onmessage = async (e: MessageEvent) => {
	const { type } = e.data;

	if (type === "derive") {
		const { password, salt, time, memory, parallelism } = e.data;
		const saltBytes = Uint8Array.from(atob(salt), (c) => c.charCodeAt(0));

		// Argon2id -> Master Key
		const result = await argon2.hash({
			pass: password,
			salt: saltBytes,
			time,
			mem: memory,
			hashLen: 32,
			parallelism,
			type: argon2.ArgonType.Argon2id,
		});
		const masterKey = new Uint8Array(result.hash);

		// HKDF(Master Key, "forged-stretch") -> Stretched Master Key
		const hkdfKey = await crypto.subtle.importKey("raw", masterKey, "HKDF", false, ["deriveBits"]);
		const stretchedBits = await crypto.subtle.deriveBits(
			{
				name: "HKDF",
				hash: "SHA-256",
				salt: new Uint8Array(0),
				info: new TextEncoder().encode("forged-stretch"),
			},
			hkdfKey,
			256
		);
		stretchedMasterKey = new Uint8Array(stretchedBits);

		// PBKDF2(Master Key, password, 1) -> Master Password Hash
		const pbkdfKey = await crypto.subtle.importKey("raw", masterKey, "PBKDF2", false, ["deriveBits"]);
		const hashBits = await crypto.subtle.deriveBits(
			{
				name: "PBKDF2",
				hash: "SHA-256",
				salt: new TextEncoder().encode(password),
				iterations: 1,
			},
			pbkdfKey,
			256
		);

		// Zero master key
		masterKey.fill(0);

		self.postMessage({
			type: "hash",
			masterPasswordHash: new Uint8Array(hashBits),
		});
	}

	if (type === "decrypt") {
		const { protectedSymmetricKey } = e.data;

		if (!stretchedMasterKey) {
			self.postMessage({ type: "error", error: "no stretched key available" });
			return;
		}

		try {
			const combined = Uint8Array.from(atob(protectedSymmetricKey), (c) => c.charCodeAt(0));
			const nonce = combined.slice(0, 12);
			const ciphertext = combined.slice(12);

			const aesKey = await crypto.subtle.importKey("raw", stretchedMasterKey.buffer as ArrayBuffer, "AES-GCM", false, ["decrypt"]);
			const symmetricKeyBytes = new Uint8Array(await crypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, aesKey, ciphertext));

			// Import as non-extractable CryptoKey inside Worker
			const cryptoKey = await crypto.subtle.importKey("raw", symmetricKeyBytes, "AES-GCM", false, ["encrypt", "decrypt"]);

			// Zero sensitive material
			stretchedMasterKey.fill(0);
			stretchedMasterKey = null;
			symmetricKeyBytes.fill(0);

			self.postMessage({ type: "key", cryptoKey });
		} catch {
			stretchedMasterKey?.fill(0);
			stretchedMasterKey = null;
			self.postMessage({ type: "error", error: "wrong password" });
		}
	}
};
