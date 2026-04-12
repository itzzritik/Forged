const TRAILING_EQUALS = /=+$/;

function encodeSSHString(data: Uint8Array): Uint8Array {
	const buf = new Uint8Array(4 + data.length);
	new DataView(buf.buffer as ArrayBuffer).setUint32(0, data.length);
	buf.set(data, 4);
	return buf;
}

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
