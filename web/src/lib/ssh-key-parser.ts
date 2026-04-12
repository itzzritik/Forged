export interface ParsedSSHKey {
	comment: string;
	convertedToOpenSSH: boolean;
	sourceFormat: "openssh" | "pkcs8-pem" | "legacy-pem";
	privateKeyBytes: Uint8Array;
	publicKeyBlob: Uint8Array;
	type: "ed25519" | "rsa" | "ecdsa";
}

const OPENSSH_HEADER = "-----BEGIN OPENSSH PRIVATE KEY-----";
const OPENSSH_FOOTER = "-----END OPENSSH PRIVATE KEY-----";
const OPENSSH_MAGIC = "openssh-key-v1\0";
const PEM_HEADER_RE = /^-----BEGIN ([A-Z0-9 ]+)-----$/;

const OID_EC_PUBLIC_KEY = "1.2.840.10045.2.1";
const OID_NIST_P256 = "1.2.840.10045.3.1.7";
const OID_NIST_P384 = "1.3.132.0.34";
const OID_NIST_P521 = "1.3.132.0.35";
const OID_RSA_ENCRYPTION = "1.2.840.113549.1.1.1";
const OID_ED25519 = "1.3.101.112";

type ParsedPem = {
	body: Uint8Array;
	label: string;
};

type ImportSpec = {
	algorithm: AlgorithmIdentifier | EcKeyImportParams | RsaHashedImportParams;
	type: ParsedSSHKey["type"];
};

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

function detectKeyFormat(content: string): "openssh" | "pem" | "unknown" {
	const trimmed = content.trim();
	if (trimmed.startsWith(OPENSSH_HEADER)) return "openssh";
	if (trimmed.startsWith("-----BEGIN") && trimmed.includes("PRIVATE KEY-----")) return "pem";
	return "unknown";
}

function normalizePrivateKeyText(content: string): string {
	const trimmed = content.trim();
	return trimmed === "" ? "" : `${trimmed}\n`;
}

function encodePrivateKeyText(content: string): Uint8Array {
	return new TextEncoder().encode(normalizePrivateKeyText(content));
}

function concatBytes(...parts: Uint8Array[]): Uint8Array {
	const total = parts.reduce((sum, part) => sum + part.length, 0);
	const out = new Uint8Array(total);
	let offset = 0;
	for (const part of parts) {
		out.set(part, offset);
		offset += part.length;
	}
	return out;
}

function encodeLength(length: number): Uint8Array {
	if (length < 0x80) {
		return Uint8Array.of(length);
	}

	const bytes: number[] = [];
	let value = length;
	while (value > 0) {
		bytes.unshift(value & 0xff);
		value >>= 8;
	}
	return Uint8Array.of(0x80 | bytes.length, ...bytes);
}

function encodeDer(tag: number, value: Uint8Array): Uint8Array {
	return concatBytes(Uint8Array.of(tag), encodeLength(value.length), value);
}

function encodeDerSequence(...children: Uint8Array[]): Uint8Array {
	return encodeDer(0x30, concatBytes(...children));
}

function encodeDerIntegerZero(): Uint8Array {
	return Uint8Array.of(0x02, 0x01, 0x00);
}

function encodeDerNull(): Uint8Array {
	return Uint8Array.of(0x05, 0x00);
}

function encodeOidValue(oid: string): Uint8Array {
	const parts = oid.split(".").map((part) => Number.parseInt(part, 10));
	if (parts.length < 2 || parts.some((part) => Number.isNaN(part))) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const bytes: number[] = [parts[0] * 40 + parts[1]];
	for (const part of parts.slice(2)) {
		const stack = [part & 0x7f];
		let value = part >>> 7;
		while (value > 0) {
			stack.unshift((value & 0x7f) | 0x80);
			value >>>= 7;
		}
		bytes.push(...stack);
	}
	return new Uint8Array(bytes);
}

function encodeOid(oid: string): Uint8Array {
	return encodeDer(0x06, encodeOidValue(oid));
}

type DerElement = {
	length: number;
	tag: number;
	value: Uint8Array;
	nextOffset: number;
};

function readDerElement(data: Uint8Array, offset = 0): DerElement {
	if (offset >= data.length) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const tag = data[offset++];
	if (offset >= data.length) {
		throw new Error("UNKNOWN_FORMAT");
	}

	let length = data[offset++];
	if ((length & 0x80) !== 0) {
		const lengthBytes = length & 0x7f;
		if (lengthBytes === 0 || lengthBytes > 4 || offset + lengthBytes > data.length) {
			throw new Error("UNKNOWN_FORMAT");
		}
		length = 0;
		for (let i = 0; i < lengthBytes; i++) {
			length = (length << 8) | data[offset++];
		}
	}

	const end = offset + length;
	if (end > data.length) {
		throw new Error("UNKNOWN_FORMAT");
	}

	return {
		tag,
		length,
		value: data.slice(offset, end),
		nextOffset: end,
	};
}

function readDerChildren(value: Uint8Array): DerElement[] {
	const children: DerElement[] = [];
	let offset = 0;
	while (offset < value.length) {
		const child = readDerElement(value, offset);
		children.push(child);
		offset = child.nextOffset;
	}
	return children;
}

function decodeOid(value: Uint8Array): string {
	if (value.length === 0) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const first = value[0];
	const parts = [Math.floor(first / 40), first % 40];
	let current = 0;

	for (const byte of value.slice(1)) {
		current = (current << 7) | (byte & 0x7f);
		if ((byte & 0x80) === 0) {
			parts.push(current);
			current = 0;
		}
	}

	if (current !== 0) {
		throw new Error("UNKNOWN_FORMAT");
	}

	return parts.join(".");
}

function decodeBase64(data: string): Uint8Array {
	return Uint8Array.from(atob(data), (char) => char.charCodeAt(0));
}

function decodeOpenSSHBase64(content: string): Uint8Array {
	const start = content.indexOf(OPENSSH_HEADER) + OPENSSH_HEADER.length;
	const end = content.indexOf(OPENSSH_FOOTER);
	return decodeBase64(content.slice(start, end).replace(/\s/g, ""));
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

	if (keyType === "ssh-ed25519") {
		type = "ed25519";
		priv.readBytes(); // public key (32 bytes), skip
		priv.readBytes(); // private key (64 bytes: seed + public), skip
	} else if (keyType === "ssh-rsa") {
		type = "rsa";
	} else if (keyType.startsWith("ecdsa-sha2-")) {
		type = "ecdsa";
	} else {
		throw new Error(`Unsupported key type: ${keyType}`);
	}

	const comment = priv.readString();

	return {
		type,
		comment,
		convertedToOpenSSH: false,
		sourceFormat: "openssh",
		publicKeyBlob,
		privateKeyBytes: encodePrivateKeyText(content),
	};
}

function parsePemBlock(content: string): ParsedPem {
	const normalizedText = normalizePrivateKeyText(content);
	const lines = normalizedText.trim().split(/\r?\n/);
	const header = lines[0]?.match(PEM_HEADER_RE);
	if (!header) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const footerLine = `-----END ${header[1]}-----`;
	const footerIndex = lines.findIndex((line, index) => index > 0 && line === footerLine);
	if (footerIndex < 0) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const bodyLines = lines.slice(1, footerIndex);
	const headers: string[] = [];
	const base64Lines: string[] = [];
	let inHeaderSection = true;

	for (const line of bodyLines) {
		if (line.trim() === "") {
			inHeaderSection = false;
			continue;
		}
		if (inHeaderSection && line.includes(":")) {
			headers.push(line);
			continue;
		}
		inHeaderSection = false;
		base64Lines.push(line.trim());
	}

	if (header[1] === "ENCRYPTED PRIVATE KEY" || headers.some((line) => /ENCRYPTED/i.test(line))) {
		throw new Error("PASSPHRASE_PROTECTED");
	}
	if (base64Lines.length === 0) {
		throw new Error("UNKNOWN_FORMAT");
	}

	return {
		label: header[1],
		body: decodeBase64(base64Lines.join("")),
	};
}

function curveFromOid(oid: string): { namedCurve: EcKeyImportParams["namedCurve"]; sshName: string } {
	switch (oid) {
		case OID_NIST_P256:
			return { namedCurve: "P-256", sshName: "nistp256" };
		case OID_NIST_P384:
			return { namedCurve: "P-384", sshName: "nistp384" };
		case OID_NIST_P521:
			return { namedCurve: "P-521", sshName: "nistp521" };
		default:
			throw new Error("UNKNOWN_FORMAT");
	}
}

function detectPkcs8ImportSpec(pkcs8: Uint8Array): ImportSpec {
	const root = readDerElement(pkcs8);
	if (root.tag !== 0x30) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const children = readDerChildren(root.value);
	if (children.length < 3 || children[1]?.tag !== 0x30) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const algorithmChildren = readDerChildren(children[1].value);
	if (algorithmChildren.length === 0 || algorithmChildren[0]?.tag !== 0x06) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const algorithmOid = decodeOid(algorithmChildren[0].value);

	switch (algorithmOid) {
		case OID_RSA_ENCRYPTION:
			return {
				type: "rsa",
				algorithm: { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" },
			};
		case OID_EC_PUBLIC_KEY: {
			if (algorithmChildren[1]?.tag !== 0x06) {
				throw new Error("UNKNOWN_FORMAT");
			}
			const curve = curveFromOid(decodeOid(algorithmChildren[1].value));
			return {
				type: "ecdsa",
				algorithm: { name: "ECDSA", namedCurve: curve.namedCurve },
			};
		}
		case OID_ED25519:
			return {
				type: "ed25519",
				algorithm: { name: "Ed25519" },
			};
		default:
			throw new Error("UNKNOWN_FORMAT");
	}
}

function wrapPkcs1InPkcs8(pkcs1: Uint8Array): Uint8Array {
	return encodeDerSequence(encodeDerIntegerZero(), encodeDerSequence(encodeOid(OID_RSA_ENCRYPTION), encodeDerNull()), encodeDer(0x04, pkcs1));
}

function extractEcCurveOid(sec1: Uint8Array): string {
	const root = readDerElement(sec1);
	if (root.tag !== 0x30) {
		throw new Error("UNKNOWN_FORMAT");
	}

	for (const child of readDerChildren(root.value)) {
		if (child.tag !== 0xa0) {
			continue;
		}
		const oid = readDerElement(child.value);
		if (oid.tag === 0x06) {
			return decodeOid(oid.value);
		}
	}

	throw new Error("UNKNOWN_FORMAT");
}

function wrapSec1InPkcs8(sec1: Uint8Array, curveOid: string): Uint8Array {
	return encodeDerSequence(encodeDerIntegerZero(), encodeDerSequence(encodeOid(OID_EC_PUBLIC_KEY), encodeOid(curveOid)), encodeDer(0x04, sec1));
}

function decodeBase64Url(value: string): Uint8Array {
	const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
	return decodeBase64(padded);
}

function encodeUint32(value: number): Uint8Array {
	const out = new Uint8Array(4);
	new DataView(out.buffer as ArrayBuffer).setUint32(0, value);
	return out;
}

function encodeSSHString(data: Uint8Array): Uint8Array {
	return concatBytes(encodeUint32(data.length), data);
}

function encodePositiveMpint(data: Uint8Array): Uint8Array {
	let start = 0;
	while (start < data.length - 1 && data[start] === 0) {
		start++;
	}
	const normalized = data.slice(start);
	if (normalized.length === 0) {
		return normalized;
	}
	if ((normalized[0] & 0x80) === 0) {
		return normalized;
	}
	return concatBytes(Uint8Array.of(0), normalized);
}

function encodeMpint(data: Uint8Array): Uint8Array {
	return encodeSSHString(encodePositiveMpint(data));
}

function curveFromJwkCrv(crv: string): { namedCurve: EcKeyImportParams["namedCurve"]; sshName: string } {
	switch (crv) {
		case "P-256":
			return { namedCurve: "P-256", sshName: "nistp256" };
		case "P-384":
			return { namedCurve: "P-384", sshName: "nistp384" };
		case "P-521":
			return { namedCurve: "P-521", sshName: "nistp521" };
		default:
			throw new Error("UNKNOWN_FORMAT");
	}
}

function buildPublicKeyBlobFromJwk(jwk: JsonWebKey, type: ParsedSSHKey["type"]): Uint8Array {
	switch (type) {
		case "rsa": {
			if (!jwk.n || !jwk.e) {
				throw new Error("UNKNOWN_FORMAT");
			}
			return concatBytes(
				encodeSSHString(new TextEncoder().encode("ssh-rsa")),
				encodeMpint(decodeBase64Url(jwk.e)),
				encodeMpint(decodeBase64Url(jwk.n))
			);
		}
		case "ecdsa": {
			if (!jwk.crv || !jwk.x || !jwk.y) {
				throw new Error("UNKNOWN_FORMAT");
			}
			const curve = curveFromJwkCrv(jwk.crv);
			const point = concatBytes(Uint8Array.of(0x04), decodeBase64Url(jwk.x), decodeBase64Url(jwk.y));
			const keyType = new TextEncoder().encode(`ecdsa-sha2-${curve.sshName}`);
			return concatBytes(encodeSSHString(keyType), encodeSSHString(new TextEncoder().encode(curve.sshName)), encodeSSHString(point));
		}
		case "ed25519": {
			if (jwk.crv !== "Ed25519" || !jwk.x) {
				throw new Error("UNKNOWN_FORMAT");
			}
			return concatBytes(encodeSSHString(new TextEncoder().encode("ssh-ed25519")), encodeSSHString(decodeBase64Url(jwk.x)));
		}
	}
}

function buildEd25519PublicKeyBlob(publicKeyRaw: Uint8Array): Uint8Array {
	return concatBytes(encodeSSHString(new TextEncoder().encode("ssh-ed25519")), encodeSSHString(publicKeyRaw));
}

function generateOpenSSHPadding(length: number, blockSize: number): Uint8Array {
	const bytes: number[] = [];
	for (let i = 0; (length + i) % blockSize !== 0; i++) {
		bytes.push(i + 1);
	}
	return Uint8Array.from(bytes);
}

function pemEncode(label: string, bytes: Uint8Array): string {
	const body = btoa(String.fromCharCode(...bytes));
	const lines = body.match(/.{1,64}/g) ?? [];
	return `-----BEGIN ${label}-----\n${lines.join("\n")}\n-----END ${label}-----\n`;
}

function randomCheckUint32(): number {
	const bytes = crypto.getRandomValues(new Uint8Array(4));
	return new DataView(bytes.buffer as ArrayBuffer).getUint32(0);
}

function encodeOpenSSHEd25519PrivateKeyPem(seed: Uint8Array, publicKeyRaw: Uint8Array, comment: string): string {
	const commentBytes = new TextEncoder().encode(comment);
	const publicKeyBlob = buildEd25519PublicKeyBlob(publicKeyRaw);
	const check = randomCheckUint32();
	const keyTypeBytes = new TextEncoder().encode("ssh-ed25519");
	const privateKey = concatBytes(seed, publicKeyRaw);
	const keyBody = concatBytes(encodeSSHString(publicKeyRaw), encodeSSHString(privateKey), encodeSSHString(commentBytes));
	const privateBlockWithoutPad = concatBytes(encodeUint32(check), encodeUint32(check), encodeSSHString(keyTypeBytes), keyBody);
	const privateBlock = concatBytes(privateBlockWithoutPad, generateOpenSSHPadding(privateBlockWithoutPad.length, 8));
	const encoded = concatBytes(
		new TextEncoder().encode(OPENSSH_MAGIC),
		encodeSSHString(new TextEncoder().encode("none")),
		encodeSSHString(new TextEncoder().encode("none")),
		encodeSSHString(new Uint8Array()),
		encodeUint32(1),
		encodeSSHString(publicKeyBlob),
		encodeSSHString(privateBlock)
	);
	return pemEncode("OPENSSH PRIVATE KEY", encoded);
}

function encodeOpenSSHPrivateKeyPemFromJwk(jwk: JsonWebKey, type: ParsedSSHKey["type"], comment: string): string {
	const commentBytes = new TextEncoder().encode(comment);
	const publicKeyBlob = buildPublicKeyBlobFromJwk(jwk, type);
	const check = randomCheckUint32();

	let keyTypeBytes: Uint8Array;
	let keyBody: Uint8Array;

	switch (type) {
		case "rsa": {
			if (!jwk.n || !jwk.e || !jwk.d || !jwk.p || !jwk.q || !jwk.qi) {
				throw new Error("UNKNOWN_FORMAT");
			}
			keyTypeBytes = new TextEncoder().encode("ssh-rsa");
			keyBody = concatBytes(
				encodeMpint(decodeBase64Url(jwk.n)),
				encodeMpint(decodeBase64Url(jwk.e)),
				encodeMpint(decodeBase64Url(jwk.d)),
				encodeMpint(decodeBase64Url(jwk.qi)),
				encodeMpint(decodeBase64Url(jwk.p)),
				encodeMpint(decodeBase64Url(jwk.q)),
				encodeSSHString(commentBytes)
			);
			break;
		}
		case "ecdsa": {
			if (!jwk.crv || !jwk.x || !jwk.y || !jwk.d) {
				throw new Error("UNKNOWN_FORMAT");
			}
			const curve = curveFromJwkCrv(jwk.crv);
			const pub = concatBytes(Uint8Array.of(0x04), decodeBase64Url(jwk.x), decodeBase64Url(jwk.y));
			keyTypeBytes = new TextEncoder().encode(`ecdsa-sha2-${curve.sshName}`);
			keyBody = concatBytes(
				encodeSSHString(new TextEncoder().encode(curve.sshName)),
				encodeSSHString(pub),
				encodeMpint(decodeBase64Url(jwk.d)),
				encodeSSHString(commentBytes)
			);
			break;
		}
		case "ed25519": {
			if (jwk.crv !== "Ed25519" || !jwk.x || !jwk.d) {
				throw new Error("UNKNOWN_FORMAT");
			}
			const pub = decodeBase64Url(jwk.x);
			const priv = concatBytes(decodeBase64Url(jwk.d), pub);
			keyTypeBytes = new TextEncoder().encode("ssh-ed25519");
			keyBody = concatBytes(encodeSSHString(pub), encodeSSHString(priv), encodeSSHString(commentBytes));
			break;
		}
	}

	const privateBlockWithoutPad = concatBytes(encodeUint32(check), encodeUint32(check), encodeSSHString(keyTypeBytes), keyBody);
	const privateBlock = concatBytes(privateBlockWithoutPad, generateOpenSSHPadding(privateBlockWithoutPad.length, 8));
	const encoded = concatBytes(
		new TextEncoder().encode(OPENSSH_MAGIC),
		encodeSSHString(new TextEncoder().encode("none")),
		encodeSSHString(new TextEncoder().encode("none")),
		encodeSSHString(new Uint8Array()),
		encodeUint32(1),
		encodeSSHString(publicKeyBlob),
		encodeSSHString(privateBlock)
	);

	return pemEncode("OPENSSH PRIVATE KEY", encoded);
}

export async function exportPrivateKeyToOpenSSH(privateKey: CryptoKey, type: ParsedSSHKey["type"], comment = ""): Promise<Uint8Array> {
	const jwk = await crypto.subtle.exportKey("jwk", privateKey);
	return new TextEncoder().encode(encodeOpenSSHPrivateKeyPemFromJwk(jwk, type, comment));
}

function parseEd25519Pkcs8(pkcs8: Uint8Array): { privateKeySeed: Uint8Array; publicKeyRaw: Uint8Array } {
	const root = readDerElement(pkcs8);
	if (root.tag !== 0x30) {
		throw new Error("UNKNOWN_FORMAT");
	}

	const children = readDerChildren(root.value);
	if (children.length < 3 || children[2]?.tag !== 0x04) {
		throw new Error("UNKNOWN_FORMAT");
	}

	let privateKeySeed = children[2].value;
	const nested = readDerElement(privateKeySeed);
	if (nested.tag === 0x04 && nested.nextOffset === privateKeySeed.length) {
		privateKeySeed = nested.value;
	}
	if (privateKeySeed.length !== 32) {
		throw new Error("UNKNOWN_FORMAT");
	}

	for (const child of children.slice(3)) {
		if (child.tag !== 0xa1) {
			continue;
		}
		const bitString = readDerElement(child.value);
		if (bitString.tag !== 0x03 || bitString.value.length !== 33 || bitString.value[0] !== 0x00) {
			break;
		}
		return {
			privateKeySeed,
			publicKeyRaw: bitString.value.slice(1),
		};
	}

	throw new Error("UNKNOWN_FORMAT");
}

async function parsePemPrivateKey(content: string): Promise<ParsedSSHKey> {
	const pem = parsePemBlock(content);
	const sourceFormat = pem.label === "PRIVATE KEY" ? "pkcs8-pem" : "legacy-pem";

	let pkcs8 = pem.body;
	let importSpec: ImportSpec;

	switch (pem.label) {
		case "PRIVATE KEY":
			importSpec = detectPkcs8ImportSpec(pkcs8);
			break;
		case "RSA PRIVATE KEY":
			pkcs8 = wrapPkcs1InPkcs8(pem.body);
			importSpec = {
				type: "rsa",
				algorithm: { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" },
			};
			break;
		case "EC PRIVATE KEY": {
			const curveOid = extractEcCurveOid(pem.body);
			const curve = curveFromOid(curveOid);
			pkcs8 = wrapSec1InPkcs8(pem.body, curveOid);
			importSpec = {
				type: "ecdsa",
				algorithm: { name: "ECDSA", namedCurve: curve.namedCurve },
			};
			break;
		}
		default:
			throw new Error("UNKNOWN_FORMAT");
	}

	const pkcs8Bytes = Uint8Array.from(pkcs8);
	if (importSpec.type === "ed25519") {
		const { privateKeySeed, publicKeyRaw } = parseEd25519Pkcs8(pkcs8Bytes);
		const expectedPublicKeyBlob = buildEd25519PublicKeyBlob(publicKeyRaw);
		const openSSHText = encodeOpenSSHEd25519PrivateKeyPem(privateKeySeed, publicKeyRaw, "");
		const parsed = parseOpenSSHKey(openSSHText);

		if (
			parsed.publicKeyBlob.length !== expectedPublicKeyBlob.length ||
			parsed.publicKeyBlob.some((byte, index) => byte !== expectedPublicKeyBlob[index])
		) {
			throw new Error("UNKNOWN_FORMAT");
		}

		return {
			...parsed,
			comment: "",
			convertedToOpenSSH: true,
			sourceFormat,
		};
	}

	const privateKey = await crypto.subtle.importKey("pkcs8", pkcs8Bytes, importSpec.algorithm, true, ["sign"]);
	const jwk = await crypto.subtle.exportKey("jwk", privateKey);
	const expectedPublicKeyBlob = buildPublicKeyBlobFromJwk(jwk, importSpec.type);
	const openSSHText = encodeOpenSSHPrivateKeyPemFromJwk(jwk, importSpec.type, "");
	const parsed = parseOpenSSHKey(openSSHText);

	if (parsed.type !== importSpec.type) {
		throw new Error("UNKNOWN_FORMAT");
	}
	if (parsed.publicKeyBlob.length !== expectedPublicKeyBlob.length || parsed.publicKeyBlob.some((byte, index) => byte !== expectedPublicKeyBlob[index])) {
		throw new Error("UNKNOWN_FORMAT");
	}

	return {
		...parsed,
		comment: "",
		convertedToOpenSSH: true,
		sourceFormat,
	};
}

export async function parseSSHKeyFile(content: string): Promise<ParsedSSHKey> {
	const format = detectKeyFormat(content);

	if (format === "unknown") {
		throw new Error("UNKNOWN_FORMAT");
	}
	if (format === "pem") {
		return parsePemPrivateKey(content);
	}

	return parseOpenSSHKey(content);
}
