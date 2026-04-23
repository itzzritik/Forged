import { cookies } from "next/headers";
import { NextResponse } from "next/server";

export const COOKIE_NAME = "forged_session";
const THIRTY_DAYS = 60 * 60 * 24 * 30;
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type SessionUser = {
	id: string;
	email: string;
	name: string;
};

export type AuthSession = {
	accessToken: string;
	accessExpiresAt: string;
	refreshToken: string;
	refreshExpiresAt: string;
	user: SessionUser;
};

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

async function encrypt(plaintext: string): Promise<string> {
	const key = await importKey("encrypt");
	const iv = crypto.getRandomValues(new Uint8Array(12));
	const encoded = new TextEncoder().encode(plaintext);
	const ciphertext = new Uint8Array(await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, encoded));
	const combined = new Uint8Array(iv.length + ciphertext.length);
	combined.set(iv, 0);
	combined.set(ciphertext, iv.length);
	return toBase64(combined);
}

async function decrypt(encrypted: string): Promise<string | null> {
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

function sessionCookieOptions() {
	return {
		httpOnly: true,
		secure: process.env.NODE_ENV === "production",
		sameSite: "lax" as const,
		path: "/",
		maxAge: THIRTY_DAYS,
	};
}

export function sessionFromQuery(params: URLSearchParams): AuthSession | null {
	const accessToken = params.get("access_token") || "";
	const accessExpiresAt = params.get("access_expires_at") || "";
	const refreshToken = params.get("refresh_token") || "";
	const refreshExpiresAt = params.get("refresh_expires_at") || "";
	const userID = params.get("user_id") || "";
	const email = params.get("email") || "";
	const name = params.get("name") || "";
	if (!accessToken || !accessExpiresAt || !refreshToken || !refreshExpiresAt || !userID || !email) {
		return null;
	}
	return {
		accessToken,
		accessExpiresAt,
		refreshToken,
		refreshExpiresAt,
		user: {
			id: userID,
			email,
			name,
		},
	};
}

export async function encodeSessionCookie(session: AuthSession): Promise<string> {
	return encrypt(JSON.stringify(session));
}

export async function decodeSessionCookie(value: string): Promise<AuthSession | null> {
	const decrypted = await decrypt(value);
	if (!decrypted) return null;
	try {
		const parsed = JSON.parse(decrypted);
		if (
			typeof parsed?.accessToken !== "string" ||
			typeof parsed?.accessExpiresAt !== "string" ||
			typeof parsed?.refreshToken !== "string" ||
			typeof parsed?.refreshExpiresAt !== "string" ||
			typeof parsed?.user?.id !== "string" ||
			typeof parsed?.user?.email !== "string"
		) {
			return null;
		}
		return {
			accessToken: parsed.accessToken,
			accessExpiresAt: parsed.accessExpiresAt,
			refreshToken: parsed.refreshToken,
			refreshExpiresAt: parsed.refreshExpiresAt,
			user: {
				id: parsed.user.id,
				email: parsed.user.email,
				name: typeof parsed.user.name === "string" ? parsed.user.name : "",
			},
		};
	} catch {
		return null;
	}
}

export async function setSessionCookie(session: AuthSession) {
	const encrypted = await encodeSessionCookie(session);
	const cookieStore = await cookies();
	cookieStore.set(COOKIE_NAME, encrypted, sessionCookieOptions());
}

export async function setSessionCookieOnResponse(response: NextResponse, session: AuthSession) {
	const encrypted = await encodeSessionCookie(session);
	response.cookies.set(COOKIE_NAME, encrypted, sessionCookieOptions());
}

export async function getSession(): Promise<AuthSession | null> {
	const cookieStore = await cookies();
	const cookie = cookieStore.get(COOKIE_NAME);
	if (!cookie?.value) return null;
	return decodeSessionCookie(cookie.value);
}

export async function clearSessionCookie() {
	const cookieStore = await cookies();
	cookieStore.delete(COOKIE_NAME);
}

export function accessExpired(session: AuthSession, now = Date.now()): boolean {
	const expiry = Date.parse(session.accessExpiresAt);
	return Number.isNaN(expiry) || expiry <= now;
}

export function refreshExpired(session: AuthSession, now = Date.now()): boolean {
	const expiry = Date.parse(session.refreshExpiresAt);
	return Number.isNaN(expiry) || expiry <= now;
}

export async function refreshSession(session: AuthSession): Promise<AuthSession | null> {
	if (refreshExpired(session)) {
		return null;
	}
	const resp = await fetch(`${API_URL}/api/v1/auth/refresh`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ refresh_token: session.refreshToken }),
		cache: "no-store",
	});
	if (!resp.ok) {
		return null;
	}

	const data = await resp.json();
	if (
		typeof data?.access_token !== "string" ||
		typeof data?.access_expires_at !== "string" ||
		typeof data?.refresh_token !== "string" ||
		typeof data?.refresh_expires_at !== "string"
	) {
		return null;
	}

	return {
		accessToken: data.access_token,
		accessExpiresAt: data.access_expires_at,
		refreshToken: data.refresh_token,
		refreshExpiresAt: data.refresh_expires_at,
		user: {
			id: typeof data?.user_id === "string" ? data.user_id : session.user.id,
			email: typeof data?.email === "string" ? data.email : session.user.email,
			name: typeof data?.name === "string" ? data.name : session.user.name,
		},
	};
}

export async function getAccessTokenForRequest(): Promise<{ token: string | null; session: AuthSession | null; refreshed: AuthSession | null }> {
	const session = await getSession();
	if (!session) {
		return { token: null, session: null, refreshed: null };
	}
	if (!accessExpired(session)) {
		return { token: session.accessToken, session, refreshed: null };
	}
	const refreshed = await refreshSession(session);
	if (!refreshed) {
		return { token: null, session, refreshed: null };
	}
	return { token: refreshed.accessToken, session, refreshed };
}
