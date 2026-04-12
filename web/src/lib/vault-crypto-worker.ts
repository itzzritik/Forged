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

		// Zero master key
		masterKey.fill(0);

		self.postMessage({ type: "derived" });
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

	if (type === "rekey") {
		const { oldPassword, oldSalt, oldTime, oldMemory, oldParallelism, oldProtectedKey, newPassword, newSalt, newTime, newMemory, newParallelism } = e.data;

		try {
			// Derive old stretched key
			const oldSaltBytes = Uint8Array.from(atob(oldSalt), (c) => c.charCodeAt(0));
			const oldResult = await argon2.hash({
				pass: oldPassword,
				salt: oldSaltBytes,
				time: oldTime,
				mem: oldMemory,
				hashLen: 32,
				parallelism: oldParallelism,
				type: argon2.ArgonType.Argon2id,
			});
			const oldMasterKey = new Uint8Array(oldResult.hash);
			const oldHkdf = await crypto.subtle.importKey("raw", oldMasterKey, "HKDF", false, ["deriveBits"]);
			const oldStretched = new Uint8Array(
				await crypto.subtle.deriveBits({ name: "HKDF", hash: "SHA-256", salt: new Uint8Array(0), info: new TextEncoder().encode("forged-stretch") }, oldHkdf, 256)
			);
			oldMasterKey.fill(0);

			// Decrypt protected key to get raw symmetric key
			const combined = Uint8Array.from(atob(oldProtectedKey), (c) => c.charCodeAt(0));
			const oldAesKey = await crypto.subtle.importKey("raw", oldStretched, "AES-GCM", false, ["decrypt"]);
			const symmetricKeyBytes = new Uint8Array(await crypto.subtle.decrypt({ name: "AES-GCM", iv: combined.slice(0, 12) }, oldAesKey, combined.slice(12)));
			oldStretched.fill(0);

			// Derive new stretched key
			const newSaltBytes = Uint8Array.from(atob(newSalt), (c) => c.charCodeAt(0));
			const newResult = await argon2.hash({
				pass: newPassword,
				salt: newSaltBytes,
				time: newTime,
				mem: newMemory,
				hashLen: 32,
				parallelism: newParallelism,
				type: argon2.ArgonType.Argon2id,
			});
			const newMasterKey = new Uint8Array(newResult.hash);
			const newHkdf = await crypto.subtle.importKey("raw", newMasterKey, "HKDF", false, ["deriveBits"]);
			const newStretched = new Uint8Array(
				await crypto.subtle.deriveBits({ name: "HKDF", hash: "SHA-256", salt: new Uint8Array(0), info: new TextEncoder().encode("forged-stretch") }, newHkdf, 256)
			);
			newMasterKey.fill(0);

			// Re-encrypt symmetric key with new stretched key
			const newAesKey = await crypto.subtle.importKey("raw", newStretched, "AES-GCM", false, ["encrypt"]);
			const nonce = crypto.getRandomValues(new Uint8Array(12));
			const encrypted = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, newAesKey, symmetricKeyBytes));
			newStretched.fill(0);
			symmetricKeyBytes.fill(0);

			const newProtected = new Uint8Array(12 + encrypted.byteLength);
			newProtected.set(nonce);
			newProtected.set(encrypted, 12);

			self.postMessage({ type: "rekeyed", newProtectedKey: btoa(String.fromCharCode(...newProtected)) });
		} catch {
			self.postMessage({ type: "error", error: "rekey failed (wrong old password?)" });
		}
	}
};
