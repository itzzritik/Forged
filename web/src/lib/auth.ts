import { cookies } from "next/headers";

export const COOKIE_NAME = "forged_session";
const THIRTY_DAYS = 60 * 60 * 24 * 30;

let cachedSecret: ArrayBuffer | null = null;
function getSecret(): ArrayBuffer {
	if (cachedSecret) return cachedSecret;
	const secret = process.env.AUTH_SECRET;
	if (!secret) throw new Error("AUTH_SECRET is not set");
	const raw = Uint8Array.from(atob(secret), (c) => c.charCodeAt(0));
	if (raw.length < 32) throw new Error("AUTH_SECRET must be at least 32 bytes");
	cachedSecret = raw.slice(0, 32).buffer as ArrayBuffer;
	return cachedSecret;
}

const keyCache = new Map<KeyUsage, Promise<CryptoKey>>();
function importKey(usage: KeyUsage): Promise<CryptoKey> {
	let cached = keyCache.get(usage);
	if (!cached) {
		cached = crypto.subtle.importKey("raw", getSecret(), "AES-GCM", false, [usage]);
		keyCache.set(usage, cached);
	}
	return cached;
}

function toBase64(bytes: Uint8Array): string {
	let binary = "";
	for (const byte of bytes) binary += String.fromCharCode(byte);
	return btoa(binary);
}

export async function encrypt(plaintext: string): Promise<string> {
	const key = await importKey("encrypt");
	const iv = crypto.getRandomValues(new Uint8Array(12));
	const encoded = new TextEncoder().encode(plaintext);
	const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, encoded));
	const combined = new Uint8Array(iv.length + ciphertext.length);
	combined.set(iv, 0);
	combined.set(ciphertext, iv.length);
	return toBase64(combined);
}

export async function decrypt(encrypted: string): Promise<string | null> {
	try {
		const key = await importKey("decrypt");
		const combined = Uint8Array.from(atob(encrypted), (c) => c.charCodeAt(0));
		const iv = combined.slice(0, 12).buffer as ArrayBuffer;
		const ciphertext = combined.slice(12).buffer as ArrayBuffer;
		const decrypted = await crypto.subtle.decrypt({ name: "AES-GCM", iv: new Uint8Array(iv) }, key, ciphertext);
		return new TextDecoder().decode(decrypted);
	} catch {
		return null;
	}
}

export function parseJWTPayload(token: string): Record<string, unknown> | null {
	try {
		return JSON.parse(atob(token.split(".")[1]));
	} catch {
		return null;
	}
}

export async function setSessionCookie(token: string) {
	const encrypted = await encrypt(token);
	const cookieStore = await cookies();
	cookieStore.set(COOKIE_NAME, encrypted, {
		httpOnly: true,
		secure: process.env.NODE_ENV === "production",
		sameSite: "lax",
		path: "/",
		maxAge: THIRTY_DAYS,
	});
}

export async function getSession(): Promise<string | null> {
	const cookieStore = await cookies();
	const cookie = cookieStore.get(COOKIE_NAME);
	if (!cookie?.value) return null;
	return decrypt(cookie.value);
}

export async function clearSessionCookie() {
	const cookieStore = await cookies();
	cookieStore.delete(COOKIE_NAME);
}
