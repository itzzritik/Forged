"use client";

import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { cancelDerivation, decryptBlob, decryptProtectedKey, deriveStretchedKey, encryptBlob, vaultDataFromRaw, type KDFParams, type VaultData } from "@/lib/vault-crypto";
import { getBrowserDeviceId } from "@/lib/sync/device";
import { mergeThreeWayRaw } from "@/lib/sync/merge";
import { clearSyncKey, getSyncKey, storeSyncKey, touchActivity } from "@/lib/vault-store";

export type VaultStatus = "loading" | "no-vault" | "locked" | "unlocked" | "error";

interface StatusResponse {
	has_vault: boolean;
	kdf_params?: KDFParams;
	protected_symmetric_key?: string;
}

interface PullResponse {
	blob: string;
	kdf_params?: KDFParams;
	protected_symmetric_key?: string;
	version: number;
}

export interface UseVaultReturn {
	deviceId: string;
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

async function fetchVaultResponse(deviceId: string): Promise<PullResponse> {
	const res = await fetch("/api/vault/pull", {
		headers: { "X-Device-ID": deviceId },
	});
	if (res.status === 401) throw new Error("401");
	if (!res.ok) throw new Error(`Failed to pull vault: ${res.status}`);
	return res.json();
}

async function fetchVault(
	cryptoKey: CryptoKey,
	deviceId: string
): Promise<{ data: VaultData; kdfParams: KDFParams | null; protectedKey: string | null; version: number }> {
	const json = await fetchVaultResponse(deviceId);
	const blob = Uint8Array.from(atob(json.blob), (c) => c.charCodeAt(0));
	const data = await decryptBlob(cryptoKey, blob);
	return {
		data,
		kdfParams: json.kdf_params ?? null,
		protectedKey: json.protected_symmetric_key ?? null,
		version: json.version,
	};
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
	const deviceIdRef = useRef("");
	const lastSyncedBaseRawRef = useRef<string | null>(null);

	if (!deviceIdRef.current && typeof window !== "undefined") {
		deviceIdRef.current = getBrowserDeviceId();
	}

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
					const pulled = await fetchVault(stored.cryptoKey, deviceIdRef.current);
					await storeSyncKey(stored.cryptoKey);
					setVaultData(pulled.data);
					setVersion(pulled.version);
					if (pulled.kdfParams) setKdfParams(pulled.kdfParams);
					if (pulled.protectedKey) setProtectedKey(pulled.protectedKey);
					lastSyncedBaseRawRef.current = pulled.data.raw;
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

				let nextKdfParams = body.kdf_params ?? null;
				let nextProtectedKey = body.protected_symmetric_key ?? null;

				if (!(nextKdfParams && nextProtectedKey)) {
					try {
						const pulled = await fetchVaultResponse(deviceIdRef.current);
						nextKdfParams = nextKdfParams ?? pulled.kdf_params ?? null;
						nextProtectedKey = nextProtectedKey ?? pulled.protected_symmetric_key ?? null;
					} catch {
						// Fall through to explicit metadata error below.
					}
				}

				if (!(nextKdfParams && nextProtectedKey)) {
					setStatus("error");
					setError("Vault metadata missing on server. Run forged sync from a linked CLI to repair it.");
					return;
				}
				setKdfParams(nextKdfParams);
				setProtectedKey(nextProtectedKey);
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
				setError("Vault is still loading. Retry in a moment.");
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

				const pulled = await fetchVault(cryptoKey, deviceIdRef.current);
				setVaultData(pulled.data);
				setVersion(pulled.version);
				if (pulled.kdfParams) setKdfParams(pulled.kdfParams);
				if (pulled.protectedKey) setProtectedKey(pulled.protectedKey);
				lastSyncedBaseRawRef.current = pulled.data.raw;
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

			const deviceId = deviceIdRef.current;

			const pushRaw = async (raw: string, expectedVersion: number) => {
				const blob = await encryptBlob(key, raw);
				const b64 = btoa(String.fromCharCode(...blob));

				return fetch("/api/vault/push", {
					method: "POST",
					headers: {
						"Content-Type": "application/json",
						"X-Device-ID": deviceId,
					},
					body: JSON.stringify({
						blob: b64,
						kdf_params: kdfParams,
						protected_symmetric_key: protectedKey,
						expected_version: expectedVersion,
						device_id: deviceId,
					}),
				});
			};

			const res = await pushRaw(updatedData.raw, version);
			if (res.status === 409) {
				const remote = await fetchVault(key, deviceId);
				const baseRaw = lastSyncedBaseRawRef.current ?? vaultData?.raw ?? updatedData.raw;
				const mergedRaw = mergeThreeWayRaw(baseRaw, updatedData.raw, remote.data.raw, deviceId, remote.data.metadata.deviceId);
				const mergedData = vaultDataFromRaw(mergedRaw);

				const retry = await pushRaw(mergedRaw, remote.version);
				if (retry.status === 409) {
					throw new Error("Version conflict: vault changed again while retrying sync");
				}
				if (!retry.ok) {
					throw new Error(`Push failed: ${retry.status}`);
				}

				const { version: mergedVersion } = await retry.json();
				lastSyncedBaseRawRef.current = mergedRaw;
				setVersion(mergedVersion);
				setVaultData(mergedData);
				return;
			}
			if (!res.ok) throw new Error(`Push failed: ${res.status}`);

			const { version: newVersion } = await res.json();
			lastSyncedBaseRawRef.current = updatedData.raw;
			setVersion(newVersion);
			setVaultData(updatedData);
		},
		[kdfParams, protectedKey, version, vaultData]
	);

	const lock = useCallback(async () => {
		await clearSyncKey();
		symmetricKeyRef.current = null;
		lastSyncedBaseRawRef.current = null;
		setVaultData(null);
		setError(null);
		setStatus("locked");
	}, []);

	return {
		deviceId: deviceIdRef.current,
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
