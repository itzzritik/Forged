export interface ParsedSSHKey {
	comment: string;
	privateKeyBytes: Uint8Array;
	publicKeyBlob: Uint8Array;
	type: "ed25519" | "rsa" | "ecdsa";
}

const OPENSSH_HEADER = "-----BEGIN OPENSSH PRIVATE KEY-----";
const OPENSSH_FOOTER = "-----END OPENSSH PRIVATE KEY-----";
const OPENSSH_MAGIC = "openssh-key-v1\0";

class SSHBuffer {
	private readonly view: DataView;
	private readonly data: Uint8Array;
	offset = 0;

	constructor(data: Uint8Array) {
		this.data = data;
		this.view = new DataView(data.buffer as ArrayBuffer, data.byteOffset, data.byteLength);
	}

	readUint32(): number {
		const val = this.view.getUint32(this.offset);
		this.offset += 4;
		return val;
	}

	readString(): string {
		const len = this.readUint32();
		const bytes = this.data.slice(this.offset, this.offset + len);
		this.offset += len;
		return new TextDecoder().decode(bytes);
	}

	readBytes(): Uint8Array {
		const len = this.readUint32();
		const bytes = this.data.slice(this.offset, this.offset + len);
		this.offset += len;
		return bytes;
	}
}

function detectKeyFormat(content: string): "openssh" | "pem-legacy" | "unknown" {
	const trimmed = content.trim();
	if (trimmed.startsWith(OPENSSH_HEADER)) return "openssh";
	if (trimmed.startsWith("-----BEGIN") && trimmed.includes("PRIVATE KEY-----")) return "pem-legacy";
	return "unknown";
}

function decodeOpenSSHBase64(content: string): Uint8Array {
	const start = content.indexOf(OPENSSH_HEADER) + OPENSSH_HEADER.length;
	const end = content.indexOf(OPENSSH_FOOTER);
	const b64 = content.slice(start, end).replace(/\s/g, "");
	return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
}

function parseOpenSSHKey(content: string): ParsedSSHKey {
	const raw = decodeOpenSSHBase64(content);

	const magic = new TextDecoder().decode(raw.slice(0, OPENSSH_MAGIC.length));
	if (magic !== OPENSSH_MAGIC) throw new Error("Invalid OpenSSH key format");

	const buf = new SSHBuffer(raw);
	buf.offset = OPENSSH_MAGIC.length;

	const cipherName = buf.readString();
	const _kdfName = buf.readString();
	const _kdfOptions = buf.readBytes();
	const numKeys = buf.readUint32();

	if (cipherName !== "none") {
		throw new Error("PASSPHRASE_PROTECTED");
	}
	if (numKeys !== 1) throw new Error("Multi-key files not supported");

	const publicKeyBlob = buf.readBytes();
	const privateSection = buf.readBytes();
	const priv = new SSHBuffer(privateSection);

	const check1 = priv.readUint32();
	const check2 = priv.readUint32();
	if (check1 !== check2) throw new Error("Key integrity check failed");

	const keyType = priv.readString();

	let type: ParsedSSHKey["type"];
	let privateKeyBytes: Uint8Array;

	if (keyType === "ssh-ed25519") {
		type = "ed25519";
		priv.readBytes(); // public key (32 bytes), skip
		const fullPrivate = priv.readBytes(); // 64 bytes: seed (32) + public (32)
		privateKeyBytes = fullPrivate.slice(0, 32);
	} else if (keyType === "ssh-rsa") {
		type = "rsa";
		privateKeyBytes = privateSection;
	} else if (keyType.startsWith("ecdsa-sha2-")) {
		type = "ecdsa";
		privateKeyBytes = privateSection;
	} else {
		throw new Error(`Unsupported key type: ${keyType}`);
	}

	const comment = priv.readString();

	return { type, publicKeyBlob, privateKeyBytes, comment };
}

export function parseSSHKeyFile(content: string): ParsedSSHKey {
	const format = detectKeyFormat(content);

	if (format === "pem-legacy") {
		throw new Error("PEM_LEGACY");
	}
	if (format === "unknown") {
		throw new Error("UNKNOWN_FORMAT");
	}

	return parseOpenSSHKey(content);
}
