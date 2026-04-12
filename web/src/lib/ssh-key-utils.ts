const TRAILING_EQUALS = /=+$/;

function encodeSSHString(data: Uint8Array): Uint8Array {
	const buf = new Uint8Array(4 + data.length);
	new DataView(buf.buffer as ArrayBuffer).setUint32(0, data.length);
	buf.set(data, 4);
	return buf;
}

// Ed25519 only: builds SSH wire-format public key from raw 32-byte key
export function formatSSHPublicKey(publicKeyRaw: Uint8Array, comment?: string): string {
	const keyType = new TextEncoder().encode("ssh-ed25519");
	const blob = new Uint8Array([...encodeSSHString(keyType), ...encodeSSHString(publicKeyRaw)]);
	const b64 = btoa(String.fromCharCode(...blob));
	return `ssh-ed25519 ${b64}${comment ? ` ${comment}` : ""}`;
}

export async function computeFingerprint(publicKeyRaw: Uint8Array): Promise<string> {
	const keyType = new TextEncoder().encode("ssh-ed25519");
	const blob = new Uint8Array([...encodeSSHString(keyType), ...encodeSSHString(publicKeyRaw)]);
	const hash = await crypto.subtle.digest("SHA-256", blob);
	const b64 = btoa(String.fromCharCode(...new Uint8Array(hash)));
	return `SHA256:${b64.replace(TRAILING_EQUALS, "")}`;
}

// Generic: formats a pre-built SSH wire-format public key blob (works for any key type)
export function formatSSHPublicKeyFromBlob(publicKeyBlob: Uint8Array, comment?: string): string {
	const typeLen = new DataView(publicKeyBlob.buffer as ArrayBuffer, publicKeyBlob.byteOffset).getUint32(0);
	const keyType = new TextDecoder().decode(publicKeyBlob.slice(4, 4 + typeLen));
	const b64 = btoa(String.fromCharCode(...publicKeyBlob));
	return `${keyType} ${b64}${comment ? ` ${comment}` : ""}`;
}

// Generic: computes fingerprint from a pre-built SSH wire-format public key blob
export async function computeFingerprintFromBlob(publicKeyBlob: Uint8Array): Promise<string> {
	const hash = await crypto.subtle.digest("SHA-256", publicKeyBlob.buffer as ArrayBuffer);
	const b64 = btoa(String.fromCharCode(...new Uint8Array(hash)));
	return `SHA256:${b64.replace(TRAILING_EQUALS, "")}`;
}
