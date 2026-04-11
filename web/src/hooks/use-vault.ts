"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { cancelDerivation, decryptBlob, decryptProtectedKey, derivePasswordHash, type KDFParams, type VaultData } from "@/lib/vault-crypto";
import { clearSyncKey, getSyncKey, storeSyncKey, touchActivity } from "@/lib/vault-store";

export type VaultStatus = "loading" | "no-vault" | "locked" | "unlocked" | "error";

interface StatusResponse {
	has_vault: boolean;
	kdf_params?: KDFParams;
}

interface VerifyResponse {
	protected_symmetric_key: string;
}

interface VerifyErrorResponse {
	attempts_remaining?: number;
	error: string;
	locked_until?: string;
}

interface PullResponse {
	blob: string; // base64
	version: number;
}

export interface UseVaultReturn {
	attemptsRemaining: number | null;
	error: string | null;
	kdfParams: KDFParams | null;
	lock: () => Promise<void>;
	lockedUntil: string | null;
	status: VaultStatus;
	unlock: (password: string) => Promise<void>;
	vaultData: VaultData | null;
}

async function fetchAndDecryptBlob(
	cryptoKey: CryptoKey,
	cachedBlob?: Uint8Array,
	cachedVersion?: number
): Promise<{ data: VaultData; blob: Uint8Array; version: number }> {
	const res = await fetch("/api/vault/pull");
	if (res.status === 401) throw new Error("401");
	if (!res.ok) throw new Error(`Failed to pull vault: ${res.status}`);

	const json: PullResponse = await res.json();

	// use cached blob if version matches
	if (cachedBlob && cachedVersion === json.version) {
		const data = await decryptBlob(cryptoKey, cachedBlob);
		return { data, blob: cachedBlob, version: json.version };
	}

	const blob = Uint8Array.from(atob(json.blob), (c) => c.charCodeAt(0));
	const data = await decryptBlob(cryptoKey, blob);
	return { data, blob, version: json.version };
}

export const useVault = (): UseVaultReturn => {
	const router = useRouter();
	const [status, setStatus] = useState<VaultStatus>("loading");
	const [vaultData, setVaultData] = useState<VaultData | null>(null);
	const [error, setError] = useState<string | null>(null);
	const [attemptsRemaining, setAttemptsRemaining] = useState<number | null>(null);
	const [lockedUntil, setLockedUntil] = useState<string | null>(null);
	const [kdfParams, setKdfParams] = useState<KDFParams | null>(null);

	// prevent double-init in React StrictMode
	const initialized = useRef(false);

	useEffect(() => {
		if (initialized.current) return;
		initialized.current = true;

		// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: vault initialization requires complex cascading async state logic
		const init = async () => {
			// check IndexedDB for a cached key first
			const stored = await getSyncKey();
			if (stored) {
				await touchActivity();
				try {
					const { data, blob, version } = await fetchAndDecryptBlob(stored.cryptoKey, stored.cachedBlob, stored.blobVersion);
					await storeSyncKey(stored.cryptoKey, blob, version);
					setVaultData(data);
					setStatus("unlocked");
					return;
				} catch (err) {
					if (err instanceof Error && err.message === "401") {
						router.push("/login");
						return;
					}
					// cached key is stale or blob decryption failed - fall through to locked
					await clearSyncKey();
				}
			}

			// check server for vault status
			try {
				const res = await fetch("/api/vault/status");
				if (res.status === 401) {
					router.push("/login");
					return;
				}
				if (!res.ok) {
					setStatus("error");
					setError(`Failed to check vault status: ${res.status}`);
					return;
				}
				const body: StatusResponse = await res.json();
				if (!body.has_vault) {
					setStatus("no-vault");
					return;
				}
				if (body.kdf_params) setKdfParams(body.kdf_params);
				setStatus("locked");
			} catch {
				setStatus("error");
				setError("Failed to reach server");
			}
		};

		init();
	}, [router]);

	const unlock = useCallback(
		// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: vault unlock requires complex error handling and lockout state machine
		async (password: string) => {
			if (!kdfParams) {
				setStatus("error");
				setError("KDF params not loaded");
				return;
			}

			setError(null);
			setAttemptsRemaining(null);
			setLockedUntil(null);
			// Keep status as "locked" -- modal stays visible during unlock attempt.
			// Modal handles its own loading state internally.

			try {
				// phase 1: derive hash in worker
				const masterPasswordHash = await derivePasswordHash(password, kdfParams);
				const hashB64 = btoa(String.fromCharCode(...masterPasswordHash));

				// verify with server
				const verifyRes = await fetch("/api/vault/verify", {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify({ master_password_hash: hashB64 }),
				});

				if (verifyRes.status === 401) {
					router.push("/login");
					return;
				}

				if (!verifyRes.ok) {
					const errBody: VerifyErrorResponse = await verifyRes.json().catch(() => ({
						error: "Wrong password",
					}));

					if (verifyRes.status === 423) {
						setLockedUntil(errBody.locked_until ?? null);
						setError(errBody.error || "Account locked");
						setStatus("locked");
						return;
					}

					setAttemptsRemaining(errBody.attempts_remaining ?? null);
					setError(errBody.error || "Wrong password");
					setStatus("locked");
					return;
				}

				const { protected_symmetric_key }: VerifyResponse = await verifyRes.json();

				// phase 2: decrypt protected key in worker (same worker instance, which already holds the derived key)
				const cryptoKey = await decryptProtectedKey(protected_symmetric_key);

				// store in IndexedDB
				await storeSyncKey(cryptoKey);

				// fetch and decrypt blob
				const { data, blob, version } = await fetchAndDecryptBlob(cryptoKey);
				await storeSyncKey(cryptoKey, blob, version);

				setVaultData(data);
				setStatus("unlocked");
			} catch (err) {
				cancelDerivation();
				if (err instanceof Error && err.message === "401") {
					router.push("/login");
					return;
				}
				const msg = err instanceof Error ? err.message : "Unknown error";
				// Wrong password or decryption failure -> back to locked, not error
				if (msg === "wrong password" || msg.includes("decrypt") || msg.includes("Derivation")) {
					setError("Wrong password");
					setStatus("locked");
					return;
				}
				setStatus("error");
				setError(msg);
			}
		},
		[kdfParams, router]
	);

	const lock = useCallback(async () => {
		await clearSyncKey();
		setVaultData(null);
		setError(null);
		setAttemptsRemaining(null);
		setLockedUntil(null);
		setStatus("locked");
	}, []);

	return {
		status,
		vaultData,
		error,
		attemptsRemaining,
		lockedUntil,
		kdfParams,
		unlock,
		lock,
	};
};
