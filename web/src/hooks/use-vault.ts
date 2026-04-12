"use client";

import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState } from "react";
import { cancelDerivation, decryptBlob, decryptProtectedKey, deriveStretchedKey, encryptBlob, type KDFParams, type VaultData } from "@/lib/vault-crypto";
import { clearSyncKey, getSyncKey, hasCachedKeySync, storeSyncKey, touchActivity } from "@/lib/vault-store";

export type VaultStatus = "loading" | "no-vault" | "locked" | "unlocked" | "error";

interface StatusResponse {
	has_vault: boolean;
	kdf_params?: KDFParams;
	protected_symmetric_key?: string;
}

interface PullResponse {
	blob: string;
	protected_symmetric_key?: string;
	version: number;
}

export interface UseVaultReturn {
	error: string | null;
	kdfParams: KDFParams | null;
	lock: () => Promise<void>;
	protectedKey: string | null;
	pushVault: (updatedData: VaultData) => Promise<void>;
	status: VaultStatus;
	symmetricKeyRef: React.RefObject<CryptoKey | null>;
	unlock: (password: string) => Promise<void>;
	vaultData: VaultData | null;
	version: number;
}

export const VaultContext = createContext<UseVaultReturn | null>(null);

export const useVaultContext = () => {
	const ctx = useContext(VaultContext);
	if (!ctx) throw new Error("useVaultContext must be used within DashboardShell");
	return ctx;
};

async function fetchVault(cryptoKey: CryptoKey): Promise<{ data: VaultData; version: number }> {
	const res = await fetch("/api/vault/pull");
	if (res.status === 401) throw new Error("401");
	if (!res.ok) throw new Error(`Failed to pull vault: ${res.status}`);

	const json: PullResponse = await res.json();
	const blob = Uint8Array.from(atob(json.blob), (c) => c.charCodeAt(0));
	const data = await decryptBlob(cryptoKey, blob);
	return { data, version: json.version };
}

export const useVault = (): UseVaultReturn => {
	const router = useRouter();
	const [status, setStatus] = useState<VaultStatus>("loading");
	const [vaultData, setVaultData] = useState<VaultData | null>(null);
	const [error, setError] = useState<string | null>(null);
	const [kdfParams, setKdfParams] = useState<KDFParams | null>(null);
	const [protectedKey, setProtectedKey] = useState<string | null>(null);
	const [version, setVersion] = useState(0);
	const symmetricKeyRef = useRef<CryptoKey | null>(null);

	// Sync-determine locked state before browser paints (avoids modal flash)
	useLayoutEffect(() => {
		if (!hasCachedKeySync()) setStatus("locked");
	}, []);

	// prevent double-init in React StrictMode
	const initialized = useRef(false);

	useEffect(() => {
		if (initialized.current) return;
		initialized.current = true;

		// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: vault initialization requires cascading async state logic
		const init = async () => {
			// check IndexedDB for a cached key first
			const stored = await getSyncKey();
			if (stored) {
				setStatus("loading");
				await touchActivity();
				try {
					symmetricKeyRef.current = stored.cryptoKey;
					const { data, version: v } = await fetchVault(stored.cryptoKey);
					await storeSyncKey(stored.cryptoKey);
					setVaultData(data);
					setVersion(v);
					setStatus("unlocked");
					return;
				} catch (err) {
					if (err instanceof Error && err.message === "401") {
						router.push("/login");
						return;
					}
					await clearSyncKey();
					symmetricKeyRef.current = null;
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
				if (body.protected_symmetric_key) setProtectedKey(body.protected_symmetric_key);
				setStatus("locked");
			} catch {
				setStatus("error");
				setError("Failed to reach server");
			}
		};

		init();
	}, [router]);

	const unlock = useCallback(
		async (password: string) => {
			if (kdfParams == null || protectedKey == null) {
				setStatus("error");
				setError("Vault parameters not loaded");
				return;
			}

			setError(null);

			try {
				await deriveStretchedKey(password, kdfParams);

				let cryptoKey: CryptoKey;
				try {
					cryptoKey = await decryptProtectedKey(protectedKey);
				} catch {
					setError("Wrong password");
					setStatus("locked");
					return;
				}

				symmetricKeyRef.current = cryptoKey;
				await storeSyncKey(cryptoKey);

				const { data, version: v } = await fetchVault(cryptoKey);
				setVaultData(data);
				setVersion(v);
				setStatus("unlocked");
			} catch (err) {
				cancelDerivation();
				if (err instanceof Error && err.message === "401") {
					router.push("/login");
					return;
				}
				const msg = err instanceof Error ? err.message : "Unknown error";
				setStatus("error");
				setError(msg);
			}
		},
		[kdfParams, protectedKey, router]
	);

	const pushVault = useCallback(
		async (updatedData: VaultData) => {
			const key = symmetricKeyRef.current;
			if (!key) throw new Error("Vault not unlocked");

			const blob = await encryptBlob(key, updatedData.raw);
			const b64 = btoa(String.fromCharCode(...blob));

			const res = await fetch("/api/vault/push", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					blob: b64,
					kdf_params: kdfParams,
					protected_symmetric_key: protectedKey,
					expected_version: version,
				}),
			});

			if (res.status === 409) throw new Error("Version conflict: vault was updated by another device. Please refresh.");
			if (!res.ok) throw new Error(`Push failed: ${res.status}`);

			const { version: newVersion } = await res.json();
			setVersion(newVersion);
			setVaultData(updatedData);
		},
		[kdfParams, protectedKey, version]
	);

	const lock = useCallback(async () => {
		await clearSyncKey();
		symmetricKeyRef.current = null;
		setVaultData(null);
		setError(null);
		setStatus("locked");
	}, []);

	return {
		status,
		vaultData,
		error,
		kdfParams,
		protectedKey,
		version,
		symmetricKeyRef,
		unlock,
		lock,
		pushVault,
	};
};
